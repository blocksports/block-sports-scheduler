package service

import (
	"fmt"
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

var SoccerRegex = regexp.MustCompile("Soccer")

// Time to wait for a block update until we reselect the best node
var NodeResetTime = int64(60)

var mutex = &sync.Mutex{}

func (svc *Service) InitialiseScheduler() {
	c := cron.New()

	c.AddFunc("@every 1s", svc.FetchBlockchainData)
	c.AddFunc("@every 5s", svc.FetchPriceData)
	c.AddFunc("@every 10s", svc.RecalculateMatchData)
	c.AddFunc("@every 60m", svc.FetchMatchData)

	svc.FetchMatchData()

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

		// svc.PushAppUpdates()
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

func (svc *Service) FetchMatchData() {
	var response FetchMatchDataResponse
	_, _, errs := gorequest.New().
		Get(fmt.Sprintf("https://api.the-odds-api.com/v2/sports/?apiKey=%s", OddsAPIKey)).
		EndStruct(&response)
	if errs != nil {
		err := aggregateErrors("Unable to fetch match data", errs)
		svc.Logger.Log("error", err.Error())
		svc.Logger.Log("error", response)
		return
	}

	var errors []error
	var events []OAEvent
	var navigation Navigation

	var matches []Match

	sportMatches := make(map[string][]Match)
	competitionMatches := make(map[string][]Match)

	competitions := make(map[string]*Competition)
	sports := make(map[string]*Sport)

	competitionOverview := make(map[string]CompetitionInfo)
	competitionMatched := make(map[string]float64)

	// Range over each retrieved competition/league to fetch their individual events and append event list
	for _, competition := range response.Data {
		subString := competition.Sport[0:6]
		if SoccerRegex.MatchString(subString) {
			competition.Sport = "Soccer"
		} else if !SportWhitelist[competition.Sport] {
			continue
		}

		var detailResponse FetchMatchDetailResponse
		_, _, errs := gorequest.New().
			Get(fmt.Sprintf("https://api.the-odds-api.com/v2/odds/?apiKey=%s&sport=%s&region=au", OddsAPIKey, competition.ID)).
			EndStruct(&detailResponse)
		if errs != nil {
			err := aggregateErrors("Unable to fetch match data", errs)
			errors = append(errors, err)
			// continue
			break
		}

		sportID := strings.Replace(strings.ToLower(competition.Sport), " ", "-", -1)
		competitionID := strings.Replace(strings.ToLower(competition.ID), "_", "-", -1)

		totalMatched := float64(0)
		firstDate := 9999999999

		// Iterate over each event, retrieve nav info and append to global event array
		for key, event := range detailResponse.Data.Events {
			if competitions[competition.ID] == nil {
				competitions[competition.ID] = &Competition{
					ID:    competitionID,
					Name:  competition.Name,
					Sport: competition.Sport,
					Count: 1,
				}
			} else {
				competitions[competition.ID].Count++
			}

			if sports[competition.Sport] == nil {
				sports[competition.Sport] = &Sport{
					ID:           sportID,
					Name:         competition.Sport,
					Count:        1,
					Competitions: []Competition{},
				}
			} else {
				sports[competition.Sport].Count++
			}

			numOutcomes, err := strconv.Atoi(detailResponse.Data.Info.Outcomes)
			if err != nil {
				svc.Logger.Log("error", fmt.Sprintf("Unable to parse number of outcomes for comp %s: %s", competition.ID, detailResponse.Data.Info.Outcomes))
				continue
			}

			for i, participant := range event.Participants {
				event.Participants[i] = stripCtlAndExtFromUnicode(participant)
			}

			scale := GetScale(key)

			match := Match{
				Name:            key,
				Sport:           sportID,
				CompetitionID:   event.Competition,
				CompetitionName: stripCtlAndExtFromUnicode(event.CompetitionName),
				Participants:    event.Participants,
				StartDate:       event.StartDate,
				Outcomes:        numOutcomes,
				Scale:           scale,
			}

			bestOdds := svc.GetBestOdds(match, event.Sites)

			svc.UpdateMatchData(bestOdds, &match)

			totalMatched = totalMatched + match.Matched
			date, err := strconv.Atoi(match.StartDate)
			if err != nil {
				svc.Logger.Log("error", fmt.Sprintf("Unable to parse start date into int %s: %s", competition.ID, match.StartDate))
				continue
			}

			if date < firstDate {
				firstDate = date
			}

			event.Sport = competition.Sport
			event.Name = key
			events = append(events, event)
			matches = append(matches, match)
			sportMatches[sportID] = append(sportMatches[sportID], match)
			competitionMatches[competitionID] = append(competitionMatches[competitionID], match)
		}

		compInfo := CompetitionInfo{
			ID:           competitionID,
			Name:         competition.Name,
			Sport:        competition.Sport,
			StartDate:    firstDate,
			TotalMatched: totalMatched,
		}

		competitionOverview[competitionID] = compInfo
		competitionMatched[competitionID] = totalMatched

		// If there are no matches populate differently
		if sports[competition.Sport] == nil && detailResponse.Data.Events == nil {
			sports[competition.Sport] = &Sport{
				ID:           strings.Replace(strings.ToLower(competition.Sport), " ", "-", -1),
				Name:         competition.Sport,
				Count:        0,
				Competitions: []Competition{},
			}
		} else if competitions[competition.ID] != nil {
			// Else ensure map has been set and then append
			sports[competition.Sport].Competitions = append(sports[competition.Sport].Competitions, *competitions[competition.ID])
		}
	}
	if errors != nil {
		err := aggregateErrors("Error fetch event data", errors)
		svc.Logger.Log("error", err.Error())
		return
	}

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

	err := svc.RedisSetInterface("all-matches", &matches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("sport-matches", &sportMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("competition-matches", &competitionMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("competition-detail", &competitionOverview)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("competition-amounts", &competitionMatched)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("navigation", &navigation)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	svc.Internals.SportKeys = sportKeys

	err = svc.RedisSetInterface("sport-keys", &sportKeys)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	svc.Logger.Log("msg", "Finished fetching match data")
}

func (svc *Service) RecalculateMatchData() {

	var allMatches []Match
	err := svc.RedisGetInterface("all-matches", &allMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var sportMatches map[string][]Match
	err = svc.RedisGetInterface("sport-matches", &sportMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var competitionMatches map[string][]Match
	err = svc.RedisGetInterface("competition-matches", &competitionMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var competitionDetail map[string]CompetitionInfo
	err = svc.RedisGetInterface("competition-detail", &competitionDetail)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	var competitionAmounts map[string]float64
	err = svc.RedisGetInterface("competition-amounts", &competitionAmounts)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	start := time.Now()

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

	err = svc.RedisSetInterface("all-matches", &allMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("sport-matches", &sportMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("competition-matches", &competitionMatches)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("competition-detail", &competitionDetail)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	err = svc.RedisSetInterface("competition-amounts", &competitionAmounts)
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	elapsed := time.Since(start)
	svc.Logger.Log("test", fmt.Sprintf("loop took %s", elapsed))

}

func FindBestOdds(match Match) BestOdds {
	backOdds := match.MatchOdds.Back
	var bestBackOdds []float64
	if len(backOdds) > 0 {
		for _, value := range backOdds {
			if len(value) > 0 {
				bestBackOdds = append(bestBackOdds, value[0].Odds)
			}
		}
	}

	layOdds := match.MatchOdds.Lay
	var bestLayOdds []float64
	if len(layOdds) > 0 {
		for _, value := range layOdds {
			if len(value) > 0 {
				bestLayOdds = append(bestLayOdds, value[0].Odds)
			}
		}
	}

	bestOdds := BestOdds{
		Back: bestBackOdds,
		Lay:  bestLayOdds,
	}

	return bestOdds
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
