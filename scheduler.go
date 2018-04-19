package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/parnurzeal/gorequest"
	"github.com/robfig/cron"
)

var OddsAPIKey = os.Getenv("ODDS_API_KEY")

var JSONOddsAPIKey = os.Getenv("JSON_ODDS_API_KEY")

var SoccerRegex = regexp.MustCompile("Soccer")

// Time to wait for a block update until we reselect the best node
var NodeResetTime = int64(60)

// Arbitrary date time
var firstDate = 9999999999

var mutex = &sync.Mutex{}

func (svc *Service) InitialiseScheduler() {
	c := cron.New()

	c.AddFunc("@every 1s", svc.FetchBlockchainData)
	c.AddFunc("@every 5s", svc.FetchPriceData)
	c.AddFunc("@every 10s", svc.RecalculateMatchData)
	c.AddFunc("@every 15m", svc.FetchEventData)

	svc.FetchPriceData()
	svc.FetchEventData()

	c.Start()

	svc.Cron = c
}

// Fetches the current NEO block height and updates blockchain data
func (svc *Service) FetchBlockchainData() {
	mutex.Lock()

	svc.Internals.DebugCount++ // Debug count if chain does not update

	newHeight, err := svc.NeoClient.GetBlockCount()
	if err != nil {
		svc.Logger.Log("error", fmt.Sprintf("Unable to fetch block height: %v", err))
		svc.NeoClient.SelectBestNode() // Reselect best node
	}

	if newHeight > svc.Internals.BlockHeight {
		err = svc.UpdateBlockHeight(newHeight)
		if err != nil {
			svc.Logger.Log("error", err.Error())
			return
		}

		svc.PushAppUpdates()
		svc.Internals.DebugCount = 0
	} else if svc.Internals.DebugCount > NodeResetTime {
		svc.NeoClient.SelectBestNode() // Reselect best node
		svc.Internals.DebugCount = 0
	}

	mutex.Unlock()
}

func (svc *Service) FetchPriceData() {
	var response map[string]Currency

	err := GetCurrencyRequest(&response)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	exchangeRate := 1 / response["GAS"]["USD"]

	data := PriceData{
		CurrencyData: response,
		ExchangeRate: exchangeRate,
	}

	svc.Internals.PriceDetails = data

	err = svc.SetRedis("price_data", &data)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

}

func (svc *Service) FetchEventData() {
	var response []EventData
	_, _, errs := gorequest.New().
		Get(fmt.Sprintf("https://jsonodds.com/api/odds/?oddType=game")).
		Set("x-api-key", JSONOddsAPIKey).
		EndStruct(&response)
	if errs != nil {
		err := aggregateErrors("Unable to fetch match data", errs)
		svc.Logger.Log("error", err.Error())
		return
	}

	responseJSON, _ := json.Marshal(response)
	err := ioutil.WriteFile("event-list.json", responseJSON, 0644)
	if err != nil {
		svc.Logger.Log("error", fmt.Sprintf("Unable to write match list to file: %s", err.Error()))
		return
	}

	var matches []Match
	var navigation Navigation

	sportMatches := make(map[string][]Match)
	competitionMatches := make(map[string][]Match)

	competitions := make(map[string]*Competition)
	sports := make(map[string]*Sport)

	competitionOverview := make(map[string]*CompetitionInfo)
	competitionMatched := make(map[string]float64)

	for _, event := range response {

		// Skip if we haven't whitelisted the sport/league
		if _, ok := SportDetailMap[event.Sport]; !ok {
			continue
		}

		sportDetail := SportDetailMap[event.Sport]

		if sportDetail.SportID == "soccer" {
			sportDetail.Competition = stripCtlAndExtFromUnicode(event.League)
			sportDetail.CompetitionID = strings.Replace(strings.ToLower(event.League), " ", "-", -1)
		} else if sportDetail.Competition == "" {
			// Mark for later
			sportDetail.Competition = sportDetail.Sport
			sportDetail.CompetitionID = sportDetail.SportID
		}

		event.Home = stripCtlAndExtFromUnicode(event.Home)
		event.Away = stripCtlAndExtFromUnicode(event.Away)

		name := event.Home + string("_") + event.Away

		numOutcomes := 3
		if event.Odds[0].DrawOdds == "0" {
			numOutcomes = 2
		}

		participants := []string{event.Home, event.Away}

		date, err := time.Parse(time.RFC3339, event.MatchTime+string("Z"))
		if err != nil {
			svc.Logger.Log("error", fmt.Sprintf("Unable to parse match time: %s : %s", event.MatchTime, err.Error()))
			return
		}

		startDate := strconv.FormatInt(date.Unix(), 10)

		scale := GetScale(event.ID)

		match := Match{
			Name:            name,
			Sport:           sportDetail.SportID,
			CompetitionID:   sportDetail.CompetitionID,
			CompetitionName: sportDetail.Competition,
			Participants:    participants,
			StartDate:       startDate,
			Outcomes:        numOutcomes,
			Scale:           scale,
		}

		bestOdds := GetOdds(match, event.Odds[0])

		svc.UpdateMatchData(bestOdds, &match)

		matches = append(matches, match)
		sportMatches[sportDetail.SportID] = append(sportMatches[sportDetail.SportID], match)
		competitionMatches[sportDetail.CompetitionID] = append(competitionMatches[sportDetail.CompetitionID], match)

		// Add to competition list
		if _, ok := competitions[sportDetail.CompetitionID]; ok {
			competitions[sportDetail.CompetitionID].Count++
		} else {
			competitions[sportDetail.CompetitionID] = &Competition{
				ID:    sportDetail.CompetitionID,
				Name:  sportDetail.Competition,
				Sport: sportDetail.Sport,
				Count: 1,
			}
		}

		// Add to sport list
		if _, ok := sports[sportDetail.Sport]; ok {
			sports[sportDetail.Sport].Count++
		} else {
			sports[sportDetail.Sport] = &Sport{
				ID:           sportDetail.SportID,
				Name:         sportDetail.Sport,
				Count:        1,
				Competitions: []Competition{},
			}
		}

		// Add to competition overview list
		if _, ok := competitionOverview[sportDetail.CompetitionID]; ok {
			competitionOverview[sportDetail.CompetitionID].TotalMatched += match.Matched
			if competitionOverview[sportDetail.CompetitionID].StartDate > int(date.Unix()) {
				competitionOverview[sportDetail.CompetitionID].StartDate = int(date.Unix())
			}

			competitionMatched[sportDetail.CompetitionID] += match.Matched
		} else {
			competitionOverview[sportDetail.CompetitionID] = &CompetitionInfo{
				ID:           sportDetail.CompetitionID,
				Name:         sportDetail.Competition,
				Sport:        sportDetail.Sport,
				StartDate:    int(date.Unix()),
				TotalMatched: match.Matched,
			}

			competitionMatched[sportDetail.CompetitionID] = match.Matched
		}

	}

	// Append competitions for navigation
	for _, competition := range competitions {
		sports[competition.Sport].Competitions = append(sports[competition.Sport].Competitions, *competition)
	}

	// Not the greatest solution :~)
	var sportKeys []SportKey
	for _, sport := range sports {
		navigation.Sports = append(navigation.Sports, *sport)
		var index int
		// In case we get sports that we haven't indexed we have to do this
		if idx, ok := SportOrder[sport.ID]; ok {
			index = idx
		} else {
			index = 99
		}

		key := SportKey{
			Sport: sport.ID,
			Index: index,
		}

		sportKeys = append(sportKeys, key)
	}

	sort.Sort(SportByKey(sportKeys))
	sort.Sort(BySportIndex(navigation.Sports))

	svc.Internals.SportKeys = sportKeys

	err = svc.SetRedis("all-matches", &matches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("sport-matches", &sportMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("competition-matches", &competitionMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("competition-detail", &competitionOverview)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("competition-amounts", &competitionMatched)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("navigation", &navigation)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("sport-keys", &sportKeys)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	svc.Logger.Log("msg", "Finished fetching match data")
}

func (svc *Service) RecalculateMatchData() {

	var allMatches []Match
	err := svc.GetRedis("all-matches", &allMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var sportMatches map[string][]Match
	err = svc.GetRedis("sport-matches", &sportMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var competitionMatches map[string][]Match
	err = svc.GetRedis("competition-matches", &competitionMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var competitionDetail map[string]CompetitionInfo
	err = svc.GetRedis("competition-detail", &competitionDetail)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var competitionAmounts map[string]float64
	err = svc.GetRedis("competition-amounts", &competitionAmounts)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	for competition, matches := range competitionMatches {
		if len(matches) < 1 {
			return
		}

		totalMatched := float64(0)
		firstDate := 9999999999

		compIDRaw := matches[0].CompetitionID
		compID := strings.Replace(strings.ToLower(compIDRaw), "_", "-", -1)
		compName := matches[0].CompetitionName
		compSportRaw := matches[0].Sport
		compSport := strings.Replace(strings.ToLower(compSportRaw), " ", "-", -1)

		for key, match := range matches {
			totalMatched += match.Matched
			date, err := strconv.Atoi(match.StartDate)
			if err != nil {
				svc.Logger.Log("error", fmt.Sprintf("Unable to parse start date into int %s: %s", competition, match.StartDate))
				continue
			}

			if date < firstDate {
				firstDate = date
			}

			bestOdds := FindBestOdds(match)
			svc.UpdateMatchData(bestOdds, &match)
			matches[key] = match
		}

		compInfo := CompetitionInfo{
			ID:           compID,
			Name:         compName,
			Sport:        compSport,
			StartDate:    firstDate,
			TotalMatched: totalMatched,
		}

		competitionDetail[competition] = compInfo
		competitionAmounts[competition] = totalMatched
		competitionMatches[competition] = matches
	}

	for key, match := range allMatches {
		bestOdds := FindBestOdds(match)
		svc.UpdateMatchData(bestOdds, &match)
		allMatches[key] = match
	}

	for sport, matches := range sportMatches {
		for key, match := range matches {
			bestOdds := FindBestOdds(match)
			svc.UpdateMatchData(bestOdds, &match)
			matches[key] = match
		}

		sportMatches[sport] = matches
	}

	err = svc.SetRedis("all-matches", &allMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("sport-matches", &sportMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("competition-matches", &competitionMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("competition-detail", &competitionDetail)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.SetRedis("competition-amounts", &competitionAmounts)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

}

func GetCurrencyRequest(response *map[string]Currency) error {
	_, _, errs := gorequest.New().
		Get("https://min-api.cryptocompare.com/data/pricemulti?fsyms=NEO,GAS&tsyms=USD,GAS,AUD").
		EndStruct(response)
	if errs != nil {
		return aggregateErrors("Unable to fetch currency data", errs)
	}

	return nil
}
