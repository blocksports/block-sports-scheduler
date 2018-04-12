package service

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type AppUpdateMessage struct {
	Currencies     map[string]Currency `json:"currencies"`
	Matches        []Match             `json:"matches"`
	BlockchainData BlockInfoResponse   `json:"blockchain_data"`
}

const MaxResult = 25

func (svc *Service) PushAppUpdates() {
	start := time.Now()

	var sportMatches map[string][]Match
	err := svc.GetRedis("sport-matches", &sportMatches)
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

	for sport, matches := range sportMatches {
		go svc.PushUpdate(matches, sport)
	}

	for competition, matches := range competitionMatches {
		sport := matches[0].Sport
		channelString := sport + "-" + competition

		go svc.PushUpdate(matches, channelString)
	}

	go svc.PushFPUpdate(sportMatches)

	elapsed := time.Since(start)
	svc.Logger.Log("test", fmt.Sprintf("loop took %s", elapsed))

	return
}

func (svc *Service) PushUpdate(matches []Match, channelString string) {
	channelDate := "markets-" + channelString + "-date"
	channelPopular := "markets-" + channelString + "-popular"

	sort.Sort(ByDate(matches))
	truncatedMatches := TruncateMatches(matches, MaxResult)

	blockchainInfo := BlockInfoResponse{
		AverageBlockTime: svc.Internals.AverageTime,
		BlockHeight:      svc.Internals.BlockHeight,
		UpdatedAt:        svc.Internals.UpdatedAt.Unix(),
	}

	messageData := AppUpdateMessage{
		Matches:        truncatedMatches,
		Currencies:     svc.Internals.PriceDetails.CurrencyData,
		BlockchainData: blockchainInfo,
	}

	err := svc.EncodeAndPush(messageData, channelDate, "app-update")
	if err != nil {
		svc.Logger.Log("error", fmt.Sprintf("Error pushing data %s: %s", channelDate, err.Error()))
		return
	}

	sort.Sort(ByPopular(matches))
	truncatedMatches = TruncateMatches(matches, MaxResult)

	messageData.Matches = truncatedMatches
	err = svc.EncodeAndPush(messageData, channelPopular, "app-update")
	if err != nil {
		svc.Logger.Log("error", fmt.Sprintf("Error pushing data %s: %s", channelPopular, err.Error()))
		return
	}

}

func (svc *Service) PushFPUpdate(matchMap map[string][]Match) {
	channelDate := "markets-date"
	channelPopular := "markets-popular"

	matches := GetFPMatches(matchMap, svc.Internals.SportKeys, "date")

	blockchainInfo := BlockInfoResponse{
		AverageBlockTime: svc.Internals.AverageTime,
		BlockHeight:      svc.Internals.BlockHeight,
		UpdatedAt:        svc.Internals.UpdatedAt.Unix(),
	}

	messageData := AppUpdateMessage{
		Matches:        matches,
		Currencies:     svc.Internals.PriceDetails.CurrencyData,
		BlockchainData: blockchainInfo,
	}

	err := svc.EncodeAndPush(messageData, channelDate, "app-update")
	if err != nil {
		svc.Logger.Log("error", fmt.Sprintf("Error pushing data %s: %s", channelDate, err.Error()))
		return
	}

	matches = GetFPMatches(matchMap, svc.Internals.SportKeys, "popular")

	messageData.Matches = matches
	err = svc.EncodeAndPush(messageData, channelPopular, "app-update")
	if err != nil {
		svc.Logger.Log("error", fmt.Sprintf("Error pushing data %s: %s", channelPopular, err.Error()))
		return
	}

	return
}

func (svc *Service) EncodeAndPush(messageData AppUpdateMessage, channelName, eventName string) (err error) {
	encodedData, err := EncodeData(messageData)
	if err != nil {
		return
	}

	count := 0

	// Retry up to 5 times if it fails, otherwise break
	for count < 5 {
		_, err = svc.PusherClient.Trigger(channelName, eventName, encodedData)
		if err != nil {
			count++
			continue
		}

		break
	}

	return
}

func EncodeData(data interface{}) (string, error) {
	var buf bytes.Buffer
	zipper := zlib.NewWriter(&buf)

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	zipper.Write(dataJSON)
	zipper.Close()

	encodedData := base64.StdEncoding.EncodeToString(buf.Bytes())
	return encodedData, nil
}
