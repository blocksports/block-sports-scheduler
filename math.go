package service

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/round"
)

var OddsDirection = map[string]float64{
	"Back": -1,
	"Lay":  1,
}

// GenerateSeededID generates a re-generateable ID seeded by the match name and date
func GenerateSeededID(matchName, matchDate string) string {
	var buffer bytes.Buffer
	buffer.WriteString(matchName)
	buffer.WriteString(matchDate)
	seed := buffer.String()

	h := md5.New()
	io.WriteString(h, seed)
	hash := h.Sum(nil)
	encoded := base64.StdEncoding.EncodeToString(hash[0:9])
	return strings.Replace(encoded, "/", "a", -1)
}

func GetOdds(match Match, odds EventOdds) BestOdds {
	numOutcomes := match.Outcomes
	scale := match.Scale

	fnLayDifference := makeLogistical([4]float64{186.2695, 4.2213, 29.5378, -0.07})
	layDifference := fnLayDifference(scale) / 2

	backOdds := make([]float64, numOutcomes)
	layOdds := make([]float64, numOutcomes)

	homeOdds := ConvertMoneylineOdds(odds.HomeOdds)
	awayOdds := ConvertMoneylineOdds(odds.AwayOdds)
	drawOdds := ConvertMoneylineOdds(odds.DrawOdds)

	for i := 0; i < numOutcomes; i++ {
		switch i {
		case 0:
			backOdds[i] = homeOdds
		case 1:
			backOdds[i] = awayOdds
		case 2:
			backOdds[i] = drawOdds
		}

		layOdds[i] = backOdds[i] + layDifference + addNormalNoise(layDifference)
		if layOdds[i]-backOdds[i] < 0.01 {
			layOdds[i] = backOdds[i] + 0.01
		}
	}

	return BestOdds{
		Back: backOdds,
		Lay:  layOdds,
	}
}

// GetBestOdds gets the best odds as averaged from the aggregated sites
func (svc *Service) GetBestOdds(match Match, allOdds []ThreeWayOdd) BestOdds {
	numOutcomes := match.Outcomes
	scale := match.Scale

	fnLayDifference := makeLogistical([4]float64{186.2695, 4.2213, 29.5378, -0.07})
	numSites := []float64{0, 0, 0}
	backOdds := make([]float64, numOutcomes)
	layOdds := make([]float64, numOutcomes)

	for _, odds := range allOdds {
		for i := 0; i < numOutcomes; i++ {
			var oddsString string
			switch i {
			case 0:
				oddsString = odds.HomeOdds
			case 1:
				oddsString = odds.AwayOdds
			case 2:
				oddsString = odds.DrawOdds
			}

			backFloat, err := strconv.ParseFloat(oddsString, 32)
			if err == nil {
				backOdds[i] += backFloat
				numSites[i]++
			}
		}
	}

	for i := 0; i < numOutcomes; i++ {
		backOdds[i] /= numSites[i]
		if math.IsNaN(backOdds[i]) {
			backOdds[i] = 0
			layOdds[i] = 0
			continue
		}

		layDifference := fnLayDifference(scale) / 2

		layOdds[i] = backOdds[i] + layDifference + addNormalNoise(layDifference)
		if layOdds[i]-backOdds[i] < 0.01 {
			layOdds[i] = backOdds[i] + 0.01
		}

	}

	return BestOdds{
		Back: backOdds,
		Lay:  layOdds,
	}
}

// FindBestOdds looks through an event's generated odds so their best odds can be re-used
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

func ConvertMoneylineOdds(odds string) float64 {
	floatOdds, _ := strconv.ParseFloat(odds, 64)
	if floatOdds > 0 {
		floatOdds = floatOdds/100 + 1
	} else if floatOdds < 0 {
		floatOdds = 100/-floatOdds + 1
	}

	return round.ToEven(floatOdds, 2)
}

// UpdateMatchData generates odds and matched amounts based on the current best odds, randomness and ~maths~
func (svc *Service) UpdateMatchData(bestOdds BestOdds, match *Match) error {
	exchangeRate := svc.Internals.PriceDetails.ExchangeRate
	fnTimeScale := makeSigmoidal([4]float64{1, 0.2851116, 96440480000, -19.12504})                // Grows to 1 as x -> 0
	fnMatchedLimit := makeExponential([4]float64{373247800000000000, 7.202931, 0.9016243, -5068}) // Grows to 2e7 as x -> 1
	fnNumOdds := makeLogistical([4]float64{9.9308, -3.0139, 10.8597, -1.5})

	start, err := strconv.ParseInt(match.StartDate, 10, 64)
	if err != nil {
		return err
	}

	timeTo := start - time.Now().Unix()
	if timeTo < 0 {
		timeTo = 0
	}
	timeScale := fnTimeScale(float64(timeTo))

	// If odds have not been generated yet, generate the scale
	if match.MatchOdds == nil {
		// Seed rand from match name for consistency
		hashBytes := sha1.Sum([]byte(match.Name))
		seed := binary.BigEndian.Uint64(hashBytes[:])
		source := rand.NewSource(int64(seed))
		r := rand.New(source)
		match.Scale = round.ToEven(r.Float64(), 3)
	} else if !isRandSuccess(timeScale/1.4) || timeTo == 0 {
		// Only update a percentage of times or when the match has started
		return nil
	}

	limit := math.Pow(fnMatchedLimit(match.Scale), 0.9)

	match.Matched = round.AwayFromZero(timeScale*limit*exchangeRate, 1)

	if match.Matched < 0 {
		match.Matched = 0
	}
	numOdds := fnNumOdds(timeScale+match.Scale) * 1.5
	matchOdds := svc.GenerateOdds(bestOdds, numOdds, match.Scale, timeScale)
	match.MatchOdds = &matchOdds

	return nil
}

// GenerateOdds tries to logically generate arrays of odds and available amounts based on scaling and other things
func (svc *Service) GenerateOdds(bestOdds BestOdds, numOdds, matchScale, timeScale float64) (matchOdds MatchOdds) {
	exchangeRate := svc.Internals.PriceDetails.ExchangeRate
	oddsMap := map[string][]float64{
		"Back": bestOdds.Back,
		"Lay":  bestOdds.Lay,
	}

	for oddType, bOdds := range oddsMap {
		direction := OddsDirection[oddType]

		var outcomeArray [][]Odds
		for outcome := 0; outcome < len(bOdds); outcome++ {
			numOddsInt := int(round.AwayFromZero(numOdds+addNoise(2), 0))
			if numOddsInt < 0 {
				numOddsInt = 0
			} else if numOddsInt > 7 {
				numOddsInt = 7
			}

			var oddsArray []Odds
			deferBreak := false
			for element := 0; element < numOddsInt; element++ {
				var oddsElement Odds
				if bOdds[outcome] < 200 {
					fElement := float64(element)
					scale := 0.011*math.Pow(bOdds[outcome]/1.3, 2) + 0.01*fElement*rand.Float64()/1.5
					aConstant := math.Pow(7.5*(timeScale+matchScale), 2)
					available := aConstant + math.Pow(5*math.Pow(fElement, 1.6)*(timeScale+matchScale), 2)
					odds := bOdds[outcome] + scale*direction*fElement
					if odds == 0 {
						break
					} else if odds <= 1.01 {
						odds = 1.01
						deferBreak = true
					}

					availableFinal := round.AwayFromZero((available+addNoise(available*0.8))*exchangeRate, 1)
					if availableFinal < 0.1 {
						availableFinal = 0.1
					}

					oddsElement = Odds{
						Odds:      round.AwayFromZero(odds, 2),
						Available: availableFinal,
					}
				} else {
					availableFinal := round.AwayFromZero(rand.Float64()*100*exchangeRate, 1)
					if availableFinal < 0.1 {
						availableFinal = 0.1
					}

					oddsElement = Odds{
						Odds:      200,
						Available: availableFinal,
					}
					deferBreak = true
				}

				oddsArray = append(oddsArray, oddsElement)
				if deferBreak {
					break
				}
			}

			outcomeArray = append(outcomeArray, oddsArray)
		}

		if oddType == "Back" {
			matchOdds.Back = outcomeArray
		} else {
			matchOdds.Lay = outcomeArray
		}
	}

	return
}

func makeSigmoidal(coefficients [4]float64) func(float64) float64 {
	return func(x float64) float64 {
		return (coefficients[3] + (coefficients[0]-coefficients[3])/(1+math.Pow(x/coefficients[2], coefficients[1])))
	}
}

func makeExponential(coefficients [4]float64) func(float64) float64 {
	return func(x float64) float64 {
		return coefficients[0]*math.Exp(-math.Pow(x-coefficients[1], 2)/(2*math.Pow(coefficients[2], 2))) + coefficients[3]
	}
}

func makeLogistical(coefficients [4]float64) func(float64) float64 {
	return func(x float64) float64 {
		return coefficients[0]/(1+coefficients[2]*math.Exp(coefficients[1]*x)) + coefficients[3]
	}
}

// isRandSuccess will return true with a success rate of percent (e.g. 0.25 = 25%)
func isRandSuccess(percent float64) bool {
	r := rand.Float64()
	return r <= percent
}

func addNoise(variance float64) float64 {
	r := rand.Float64()
	if isRandSuccess(0.5) {
		r *= -1
	}

	return r * variance
}

func addNormalNoise(variance float64) float64 {
	r := rand.NormFloat64()
	return r * variance
}

func GetScale(key string) float64 {
	// Seed rand from match name for consistency
	hashBytes := sha1.Sum([]byte(key))
	seed := binary.BigEndian.Uint64(hashBytes[:])
	source := rand.NewSource(int64(seed))
	r := rand.New(source)
	scale := round.ToEven(r.Float64(), 3)

	return scale
}

func GetTimeScale(unixTime int64) float64 {
	fnTimeScale := makeSigmoidal([4]float64{1, 0.2851116, 96440480000, -19.12504}) // Grows to 1 as x -> 0

	timeTo := unixTime - time.Now().Unix()
	if timeTo < 0 {
		timeTo = 0
	}
	timeScale := fnTimeScale(float64(timeTo))

	return timeScale
}
