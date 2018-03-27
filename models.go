package service

import "strconv"

type FetchMatchDataResponse struct {
	Success bool            `json:"success"`
	Data    []OACompetition `json:"data"`
}

type FetchMatchDetailResponse struct {
	Success bool        `json:"success"`
	Data    OAMatchList `json:"data"`
}

type TestResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
}
type OACompetition struct {
	ID    string `json:"sport"`
	Sport string `json:"sport_group"`
	Name  string `json:"display_name"`
}

type OAMatchList struct {
	Info   OAMatchInfo        `json:"info"`
	Events map[string]OAEvent `json:"events"`
}

type OAMatchInfo struct {
	Sport     string
	Name      string `json:"display_name"`
	Outcomes  string `json:"num_outcomes"`
	UpdatedAt int    `json:"check_dt,omitempty"`
}

type OAEvent struct {
	Name            string            `json:"name"`
	Sport           string            `json:"sport_name"`
	Competition     string            `json:"sport"`
	CompetitionName string            `json:"sport_display"`
	Participants    []string          `json:"participants"`
	StartDate       string            `json:"commence"`
	Status          string            `json:"status"`
	Sites           map[string]OASite `json:"sites"`
}

type OASite struct {
	Odds      OAOdds `json:"odds"`
	UpdatedAt int    `json:"last_update"`
}

type OAOdds struct {
	Back []string `json:"h2h"`
	Lay  []string `json:"h2h_lay,omitempty"`
}

type Navigation struct {
	Sports []Sport `json:"data"`
}

type BySportIndex []Sport

func (s BySportIndex) Len() int      { return len(s) }
func (s BySportIndex) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s BySportIndex) Less(i, j int) bool {
	var a int
	var b int
	if idx, ok := SportOrder[s[i].ID]; ok {
		a = idx
	} else {
		a = 99
	}

	if idx, ok := SportOrder[s[j].ID]; ok {
		b = idx
	} else {
		b = 99
	}

	return a < b
}

type Sport struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Count        int           `json:"count"`
	Competitions []Competition `json:"competitions"`
}

type Competition struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Sport string `json:"sport"`
	Count int    `json:"count"`
}

type CompetitionInfo struct {
	ID           string  `json:"id"`
	Sport        string  `json:"sport"`
	Name         string  `json:"name"`
	StartDate    int     `json:"commence"`
	TotalMatched float64 `json:"total_matched"`
}

type Match struct {
	Name            string     `json:"name"`
	Sport           string     `json:"sport"`
	CompetitionID   string     `json:"competition"`
	CompetitionName string     `json:"competition_name"`
	Participants    []string   `json:"participants"`
	StartDate       string     `json:"commence"`
	Outcomes        int        `json:"outcomes"`
	Matched         float64    `json:"matched"`
	MatchOdds       *MatchOdds `json:"match_odds"`
	Scale           float64    `json:"scale"` // Scale of the match : large scale == more matched etc
}

type ByDate []Match

func (m ByDate) Len() int      { return len(m) }
func (m ByDate) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m ByDate) Less(i, j int) bool {
	a, err := strconv.Atoi(m[i].StartDate)
	if err != nil {
		return false
	}

	b, err := strconv.Atoi(m[j].StartDate)
	if err != nil {
		return false
	}

	if a == b {
		a = CreateIntFromString(m[i].Name)
		b = CreateIntFromString(m[j].Name)
	}

	return a < b
}

func CreateIntFromString(str string) int {
	runeArray := []rune(str)
	output := 0
	for _, element := range runeArray {
		output *= int(element)
	}

	return output
}

type ByPopular []Match

func (m ByPopular) Len() int      { return len(m) }
func (m ByPopular) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m ByPopular) Less(i, j int) bool {
	a := m[i].Matched
	b := m[j].Matched

	return a > b
}

type BestOdds struct {
	Back []float64 `json:"back"`
	Lay  []float64 `json:"lay"`
}

type MatchOdds struct {
	Back [][]Odds `json:"back"`
	Lay  [][]Odds `json:"lay"`
}

type Odds struct {
	Odds      float64 `json:"odds"`
	Available float64 `json:"available"`
}

type SportKey struct {
	Sport string
	Index int
}

type SportByKey []SportKey

func (s SportByKey) Len() int      { return len(s) }
func (s SportByKey) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s SportByKey) Less(i, j int) bool {
	a := s[i].Index
	b := s[j].Index

	return a < b
}

var SportOrder = map[string]int{
	"soccer":             1,
	"american-football":  2,
	"mixed-martial-arts": 3,
	"basketball":         4,
	"cricket":            5,
	"ice-hockey":         6,
	"boxing":             7,
}

type BlockInfoResponse struct {
	AverageBlockTime float64 `json:"average_time"`
	BlockHeight      int64   `json:"block_height"`
	UpdatedAt        int64   `json:"updated_at"`
}
