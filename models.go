package service

import (
	"sort"
	"strconv"
)

type EventData struct {
	ID        string      `json:"ID"`
	Home      string      `json:"HomeTeam"`
	Away      string      `json:"AwayTeam"`
	MatchTime string      `json:"MatchTime"`
	League    string      `json:"DisplayLeague"`
	Sport     int         `json:"Sport"`
	Odds      []EventOdds `json:"Odds"`
}

type EventOdds struct {
	HomeOdds string `json:"MoneyLineHome"`
	AwayOdds string `json:"MoneyLineAway"`
	DrawOdds string `json:"DrawLine"`
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

type ByAlphabetical []Competition

func (c ByAlphabetical) Len() int      { return len(c) }
func (c ByAlphabetical) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c ByAlphabetical) Less(i, j int) bool {
	a := c[i].ID
	b := c[j].ID

	AB := []string{a, b}

	sort.Strings(AB)

	return AB[0] == a
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
	"soccer":            1,
	"american-football": 2,
	"mma":               3,
	"basketball":        4,
	"cricket":           5,
	"baseball":          6,
	"ice-hockey":        7,
	"boxing":            8,
	"rugby-union":       9,
}

type BlockInfoResponse struct {
	AverageBlockTime float64 `json:"average_time"`
	BlockHeight      int64   `json:"block_height"`
	UpdatedAt        int64   `json:"updated_at"`
}

type SportDetail struct {
	Sport         string
	SportID       string
	Competition   string
	CompetitionID string
}

var SportDetailMap = map[int]SportDetail{
	0: SportDetail{
		Sport:         "Baseball",
		SportID:       "baseball",
		Competition:   "MLB",
		CompetitionID: "mlb",
	},
	1: SportDetail{
		Sport:         "Basketball",
		SportID:       "basketball",
		Competition:   "NBA",
		CompetitionID: "nba",
	},
	2: SportDetail{
		Sport:         "Basketball",
		SportID:       "basketball",
		Competition:   "NCAAB",
		CompetitionID: "ncaab",
	},
	3: SportDetail{
		Sport:         "American Football",
		SportID:       "american-football",
		Competition:   "NCAAF",
		CompetitionID: "ncaaf",
	},
	4: SportDetail{
		Sport:         "American Football",
		SportID:       "american-football",
		Competition:   "NFL",
		CompetitionID: "nfl",
	},
	5: SportDetail{
		Sport:         "Ice Hockey",
		SportID:       "ice-hockey",
		Competition:   "NHL",
		CompetitionID: "nhl",
	},
	7: SportDetail{
		Sport:         "Soccer",
		SportID:       "soccer",
		Competition:   "",
		CompetitionID: "",
	},
	8: SportDetail{
		Sport:         "Basketball",
		SportID:       "basketball",
		Competition:   "WNBA",
		CompetitionID: "wnba",
	},
	9: SportDetail{
		Sport:         "Tennis",
		SportID:       "tennis",
		Competition:   "",
		CompetitionID: "",
	},
	10: SportDetail{
		Sport:         "Boxing",
		SportID:       "boxing",
		Competition:   "",
		CompetitionID: "",
	},
	11: SportDetail{
		Sport:         "Mixed Martial Arts",
		SportID:       "mma",
		Competition:   "",
		CompetitionID: "",
	},
	12: SportDetail{
		Sport:         "Cricket",
		SportID:       "cricket",
		Competition:   "",
		CompetitionID: "",
	},
	14: SportDetail{
		Sport:         "Ice Hockey",
		SportID:       "ice-hockey",
		Competition:   "KHL",
		CompetitionID: "khl",
	},
	15: SportDetail{
		Sport:         "Ice Hockey",
		SportID:       "ice-hockey",
		Competition:   "AHL",
		CompetitionID: "ahl",
	},
	16: SportDetail{
		Sport:         "Ice Hockey",
		SportID:       "ice-hockey",
		Competition:   "SHL",
		CompetitionID: "shl",
	},
	18: SportDetail{
		Sport:         "Baseball",
		SportID:       "baseball",
		Competition:   "LMP",
		CompetitionID: "lmp",
	},
	19: SportDetail{
		Sport:         "Baseball",
		SportID:       "baseball",
		Competition:   "NPB",
		CompetitionID: "npb",
	},
	20: SportDetail{
		Sport:         "Baseball",
		SportID:       "baseball",
		Competition:   "KBO",
		CompetitionID: "kbo",
	},
	22: SportDetail{
		Sport:         "Rugby Union",
		SportID:       "rugby-union",
		Competition:   "",
		CompetitionID: "",
	},
	23: SportDetail{
		Sport:         "Baseball",
		SportID:       "baseball",
		Competition:   "WBC",
		CompetitionID: "wbc",
	},
	24: SportDetail{
		Sport:         "American Football",
		SportID:       "american-football",
		Competition:   "CFL",
		CompetitionID: "cfl",
	},
}

var LeagueWhitelist = map[string]bool{
	"english-premier-league": true,
	"australian-a-league":    true,
	"2018-fifa-world-cup":    true,
	"italian-serie-a":        true,
	"south-korean-k-league":  true,
	"english-league-1":       true,
	"english-league-2":       true,
	"j-league":               true,
	"major-league-soccer":    true,
	"french-ligue-1":         true,
	"bundesliga":             true,
	"brazil-serie-a":         true,
	"spanish-la-liga":        true,
	"english-fa-cup":         true,
	"scottish-premiership":   true,
	"premier-division":       true,
}
