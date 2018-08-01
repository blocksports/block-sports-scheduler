package service

import (
	"reflect"
	"sort"
	"strconv"
)

type UpcomingEventsResponse struct {
	Success int     `json:"success"`
	Pager   Pager   `json:"pager"`
	Results []Event `json:"results"`
	Error   string  `json:"error"`
}

type EventOddsResponse struct {
	Success int          `json:"success"`
	Results ThreeWayOdds `json:"results"`
	Error   string       `json:"error"`
}

type EventOddsResponseA struct {
	Success int       `json:"success"`
	Results Providers `json:"results"`
	Error   string    `json:"error"`
}

type Event struct {
	ID        string `json:"id"`
	SportID   string `json:"sport_id"`
	MatchTime string `json:"time"`
	League    League `json:"league"`
	Home      Team   `json:"home"`
	Away      Team   `json:"away"`
}

type League struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ThreeWayOdd struct {
	HomeOdds string `json:"home_od"`
	AwayOdds string `json:"away_od"`
	DrawOdds string `json:"draw_od"`
}

type Pager struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

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

func NewSportMap() map[string]*Sport {
	sports := make(map[string]*Sport)

	for _, sport := range SportList {
		sports[sport.Name] = &Sport{
			ID:           sport.ID,
			Name:         sport.Name,
			Count:        0,
			Competitions: []Competition{},
		}
	}

	return sports
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
	Scale           float64    `json:"scale"`
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

var SportList = map[string]SportInfo{
	"1": SportInfo{
		ID:   "soccer",
		Name: "Soccer",
	},
	"3": SportInfo{
		ID:   "cricket",
		Name: "Cricket",
	},
	"8": SportInfo{
		ID:   "rugby-union",
		Name: "Rugby Union",
	},
	// "9": SportInfo{
	// 	ID:   "boxing-ufc",
	// 	Name: "Boxing/UFC",
	// },
	"12": SportInfo{
		ID:   "american-football",
		Name: "American Football",
	},
	"13": SportInfo{
		ID:   "tennis",
		Name: "Tennis",
	},
	"16": SportInfo{
		ID:   "baseball",
		Name: "Baseball",
	},
	"17": SportInfo{
		ID:   "ice-hockey",
		Name: "Ice Hockey",
	},
	"18": SportInfo{
		ID:   "basketball",
		Name: "Basketball",
	},
	"30": SportInfo{
		ID:   "esports",
		Name: "eSports",
	},
	// "19": SportInfo{
	// 	ID:   "rugby-league",
	// 	Name: "Rugby League",
	// },
}

// If we want to unmarshal cleanly we tag each individual sport
type ThreeWayOdds struct {
	SoccerOdds     []ThreeWayOdd `json:"1_1"`
	CricketOdds    []ThreeWayOdd `json:"3_1"`
	UnionOdds      []ThreeWayOdd `json:"8_1"`
	BoxingOdds     []ThreeWayOdd `json:"9_1"`
	FootballOdds   []ThreeWayOdd `json:"12_1"`
	TennisOdds     []ThreeWayOdd `json:"13_1"`
	BaseballOdds   []ThreeWayOdd `json:"16_1"`
	HockeyOdds     []ThreeWayOdd `json:"17_1"`
	BasketballOdds []ThreeWayOdd `json:"18_1"`
	LeagueOdds     []ThreeWayOdd `json:"19_1"`
}

type Providers struct {
	A1XBet       interface{} `json:"1XBet"`
	A10Bet       interface{} `json:"10Bet"`
	A888Sport    interface{} `json:"888Sport"`
	Bet365       interface{} `json:"Bet365"`
	BetAtHome    interface{} `json:"BetAtHome"`
	BetClic      interface{} `json:"BetClic"`
	Betdaq       interface{} `json:"Betdaq"`
	BetFair      interface{} `json:"BetFair"`
	BetFred      interface{} `json:"BetFred"`
	Betsson      interface{} `json:"Betsson"`
	Betway       interface{} `json:"Betway"`
	BWin         interface{} `json:"BWin"`
	Intertops    interface{} `json:"Intertops"`
	Interwetten  interface{} `json:"Interwetten"`
	Ladbrokes    interface{} `json:"Ladbrokes"`
	Marathonbet  interface{} `json:"Marathonbet"`
	MarsBet      interface{} `json:"MarsBet"`
	PlanetWin365 interface{} `json:"PlanetWin365"`
	SBOBET       interface{} `json:"SBOBET"`
	SkyBet       interface{} `json:"SkyBet"`
	TitanBet     interface{} `json:"TitanBet"`
	UniBet       interface{} `json:"UniBet"`
	VBet         interface{} `json:"VBet"`
	WilliamHill  interface{} `json:"WilliamHill"`
	Winner       interface{} `json:"Winner"`
	YSB88        interface{} `json:"YSB88"`
}

type Provider struct {
	LatestOdds ThreeWayOddsA `json:"end"`
}

func (o ThreeWayOdd) IsEmpty() bool {
	return reflect.DeepEqual(o, ThreeWayOdd{})
}

// If we want to unmarshal cleanly we tag each individual sport
type ThreeWayOddsA struct {
	SoccerOdds     ThreeWayOdd `json:"1_1"`
	CricketOdds    ThreeWayOdd `json:"3_1"`
	UnionOdds      ThreeWayOdd `json:"8_1"`
	BoxingOdds     ThreeWayOdd `json:"9_1"`
	FootballOdds   ThreeWayOdd `json:"12_1"`
	TennisOdds     ThreeWayOdd `json:"13_1"`
	BaseballOdds   ThreeWayOdd `json:"16_1"`
	HockeyOdds     ThreeWayOdd `json:"17_1"`
	BasketballOdds ThreeWayOdd `json:"18_1"`
	LeagueOdds     ThreeWayOdd `json:"19_1"`
}

func (t ThreeWayOdds) GetSportOdds(sport string) (odds []ThreeWayOdd) {
	switch s := sport; s {
	case "1":
		return t.SoccerOdds
	case "3":
		return t.CricketOdds
	case "8":
		return t.UnionOdds
	case "9":
		return t.BoxingOdds
	case "12":
		return t.FootballOdds
	case "13":
		return t.TennisOdds
	case "16":
		return t.BaseballOdds
	case "17":
		return t.HockeyOdds
	case "18":
		return t.BasketballOdds
	case "19":
		return t.LeagueOdds
	default:
		return
	}
}

func (t ThreeWayOddsA) GetSportOdds(sport string) (odds ThreeWayOdd) {
	switch s := sport; s {
	case "1":
		return t.SoccerOdds
	case "3":
		return t.CricketOdds
	case "8":
		return t.UnionOdds
	case "9":
		return t.BoxingOdds
	case "12":
		return t.FootballOdds
	case "13":
		return t.TennisOdds
	case "16":
		return t.BaseballOdds
	case "17":
		return t.HockeyOdds
	case "18":
		return t.BasketballOdds
	case "19":
		return t.LeagueOdds
	default:
		return
	}
}

type SportInfo struct {
	ID   string
	Name string
}
