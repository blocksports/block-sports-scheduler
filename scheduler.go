package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/parnurzeal/gorequest"
	"github.com/robfig/cron"
)

var APIToken = os.Getenv("SPORTS_API_TOKEN")

var OddsSources = []string{"bet365", "betfair", "10bet", "williamhill", "betclic", "ysb88", "bwin", "betfred", "betsson", "sbobet", "marathonbet", "intertops", "interwetten", "1xbet", "skybet", "marsbet"}

// Time to wait for a block update until we reselect the best node
var NodeResetTime = int64(60)

var mutex = &sync.Mutex{}

var eventMutex = &sync.Mutex{}

func (svc *Service) InitialiseScheduler() {
	svc.Logger.Log("msg", "Initialising Scheduler")

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

var a_wg = 0
var b_wg = 0

func (svc *Service) FetchEventData() {
	var wg sync.WaitGroup

	var matches []Match
	var navigation Navigation

	sportMatches := make(map[string][]Match)
	competitionMatches := make(map[string][]Match)

	competitions := make(map[string]*Competition)
	sports := NewSportMap()

	competitionOverview := make(map[string]*CompetitionInfo)
	competitionMatched := make(map[string]float64)

	// Open and read whitelist file
	file, err := os.Open("api_whitelist.csv")
	if err != nil {
		svc.Logger.Log("error", err.Error())
		return
	}

	csvR := csv.NewReader(file)

	svc.Logger.Log("msg", "Fetching match data")

	// Iterate over each row
	for {
		leagueDetail, err := csvR.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			svc.Logger.Log("error", err.Error())
			return
		}

		wg.Add(1)

		sportID := leagueDetail[0]
		leagueID := leagueDetail[1]
		leagueName := leagueDetail[2]
		leagueInternalID := leagueDetail[3]
		scaleString := leagueDetail[4]

		leagueScale, err := strconv.ParseFloat(scaleString, 64)
		if err != nil {
			svc.Logger.Log("error", err.Error())
			return
		}

		svc.Internals.LeagueScales[leagueInternalID] = leagueScale

		go func() {
			defer wg.Done()

			var response UpcomingEventsResponse
			_, _, errs := gorequest.New().
				Get(fmt.Sprintf("https://api.betsapi.com/v1/events/upcoming?sport_id=%s&league_id=%s&token=%s", sportID, leagueID, APIToken)).
				EndStruct(&response)
			if errs != nil {
				err := aggregateErrors("Unable to fetch upcoming event data", errs)
				svc.Logger.Log("error", err.Error())
				return
			} else if response.Success == 0 {
				svc.Logger.Log("error", response.Error)
				return
			}

			for _, event := range response.Results {

				var hasDraw = false
				var odds []ThreeWayOdd

				var oddsResponse EventOddsResponseA
				_, _, errs := gorequest.New().
					Get(fmt.Sprintf("https://api.betsapi.com/v1/event/odds/summary?event_id=%s&token=%s", event.ID, APIToken)).
					EndStruct(&oddsResponse)
				if errs != nil {
					// expected when there are no results

					// err := aggregateErrors(fmt.Sprintf("%s - Unable to fetch event odds", event.ID), errs)
					// svc.Logger.Log("error", err.Error())
					return
				}

				oddsReflect := reflect.ValueOf(oddsResponse.Results)

				// Iterate over all the returned providers
				// Have to re-marshal to find poorly returned objects and to cast to Provider struct
				for i := 0; i < oddsReflect.NumField(); i++ {
					rawProvider, ok := oddsReflect.Field(i).Interface().(interface{})
					if !ok {
						// expected when nil interface is given
						continue
					}

					jsonProvider, err := json.Marshal(rawProvider)
					if err != nil {
						svc.Logger.Log("error", err.Error())
						continue
					}

					var provider Provider

					err = json.Unmarshal(jsonProvider, &provider)
					if err != nil {
						// expected when array is given
						continue
					}

					latestOdds := provider.LatestOdds.GetSportOdds(sportID)

					if latestOdds.IsEmpty() {
						continue
					}

					odds = append(odds, latestOdds)
					if latestOdds.DrawOdds != "" {
						hasDraw = true
					}
				}

				if len(odds) < 1 {
					return
				}

				// Set up match details
				name := event.Home.Name + string("_") + event.Away.Name
				sport := SportList[sportID].Name
				sportID := SportList[sportID].ID
				competition := leagueName
				competitionID := leagueInternalID
				participants := []string{event.Home.Name, event.Away.Name}
				scale := leagueScale + addNoise(0.075)
				numOutcomes := 3
				if !hasDraw {
					numOutcomes = 2
				}

				match := Match{
					Name:            name,
					Sport:           sportID,
					CompetitionName: competition,
					CompetitionID:   competitionID,
					Participants:    participants,
					StartDate:       event.MatchTime,
					Outcomes:        numOutcomes,
					Scale:           scale,
				}

				bestOdds := svc.GetBestOdds(match, odds)
				_ = svc.UpdateMatchData(bestOdds, &match)

				// Add match to appropriate maps/lists
				eventMutex.Lock()

				matches = append(matches, match)
				sportMatches[sportID] = append(sportMatches[sportID], match)
				competitionMatches[competitionID] = append(competitionMatches[competitionID], match)

				// Competition list
				if _, ok := competitions[competitionID]; ok {
					competitions[competitionID].Count++
				} else {
					competitions[competitionID] = &Competition{
						ID:    competitionID,
						Name:  competition,
						Sport: sport,
						Count: 1,
					}
				}

				// Sport list
				if _, ok := sports[sport]; ok {
					sports[sport].Count++
				} else {
					svc.Logger.Log("error", fmt.Sprintf("Unknown sport name: %s", sport))
					continue
				}

				// Competition overview list
				date, _ := strconv.Atoi(event.MatchTime)
				if _, ok := competitionOverview[competitionID]; ok {
					competitionOverview[competitionID].TotalMatched += match.Matched

					if competitionOverview[competitionID].StartDate > date {
						competitionOverview[competitionID].StartDate = date
					}

					competitionMatched[competitionID] += match.Matched
				} else {
					competitionOverview[competitionID] = &CompetitionInfo{
						ID:           competitionID,
						Name:         competition,
						Sport:        sport,
						StartDate:    date,
						TotalMatched: match.Matched,
					}

					competitionMatched[competitionID] = match.Matched
				}

				eventMutex.Unlock()

			}

		}()
	}

	wg.Wait()

	// Append competitions for navigation
	for _, competition := range competitions {
		sports[competition.Sport].Competitions = append(sports[competition.Sport].Competitions, *competition)
	}

	// Not the greatest solution :~)
	var sportKeys []SportKey
	for _, sport := range sports {
		sort.Sort(ByAlphabetical(sport.Competitions))

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
