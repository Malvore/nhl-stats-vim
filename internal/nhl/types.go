package nhl

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Localized struct {
	Default string `json:"default"`
}
type Team struct {
	ID         int       `json:"id"`
	Abbrev     string    `json:"abbrev"`
	Score      *int      `json:"score"`
	Shots      *int      `json:"sog"`
	Name       Localized `json:"name"`
	CommonName Localized `json:"commonName"`
	PlaceName  Localized `json:"placeName"`
}
type Clock struct {
	Remaining    string `json:"timeRemaining"`
	Intermission bool   `json:"inIntermission"`
}
type Period struct {
	Number int    `json:"number"`
	Type   string `json:"periodType"`
}
type Game struct {
	ID            int               `json:"id"`
	GameType      int               `json:"gameType"`
	Date          string            `json:"gameDate"`
	StartUTC      time.Time         `json:"startTimeUTC"`
	State         string            `json:"gameState"`
	ScheduleState string            `json:"gameScheduleState"`
	Away          Team              `json:"awayTeam"`
	Home          Team              `json:"homeTeam"`
	Clock         Clock             `json:"clock"`
	Period        Period            `json:"periodDescriptor"`
	Venue         Localized         `json:"venue"`
	PlayerStats   PlayerByGameStats `json:"playerByGameStats"`
}

type PlayerByGameStats struct {
	Away GamePlayerGroup `json:"awayTeam"`
	Home GamePlayerGroup `json:"homeTeam"`
}
type GamePlayerGroup struct {
	Forwards []Skater `json:"forwards"`
	Defense  []Skater `json:"defense"`
	Goalies  []Goalie `json:"goalies"`
}
type Skater struct {
	ID        int       `json:"playerId"`
	Name      Localized `json:"name"`
	Number    int       `json:"sweaterNumber"`
	Position  string    `json:"position"`
	Goals     int       `json:"goals"`
	Assists   int       `json:"assists"`
	Points    int       `json:"points"`
	PlusMinus int       `json:"plusMinus"`
	PIM       int       `json:"pim"`
	Shots     int       `json:"sog"`
	Hits      int       `json:"hits"`
	Blocks    int       `json:"blockedShots"`
	TOI       string    `json:"toi"`
}
type Goalie struct {
	ID           int       `json:"playerId"`
	Name         Localized `json:"name"`
	Number       int       `json:"sweaterNumber"`
	Starter      bool      `json:"starter"`
	Saves        int       `json:"saves"`
	ShotsAgainst int       `json:"shotsAgainst"`
	GoalsAgainst int       `json:"goalsAgainst"`
	TOI          string    `json:"toi"`
}
type GameSummary struct {
	Broadcasts []struct {
		Network string `json:"network"`
		Market  string `json:"market"`
		Country string `json:"countryCode"`
	} `json:"tvBroadcasts"`
	Summary struct {
		Scoring    []PeriodSummary `json:"scoring"`
		Penalties  []PeriodSummary `json:"penalties"`
		ThreeStars []Star          `json:"threeStars"`
	} `json:"summary"`
}
type PeriodSummary struct {
	Period    Period         `json:"periodDescriptor"`
	Goals     []ScoringEvent `json:"goals"`
	Penalties []PenaltyEvent `json:"penalties"`
}
type ScoringEvent struct {
	PlayerID   int       `json:"playerId"`
	Name       Localized `json:"name"`
	Time       string    `json:"timeInPeriod"`
	TeamAbbrev Localized `json:"teamAbbrev"`
	Assists    []struct {
		Name Localized `json:"name"`
	} `json:"assists"`
}
type PenaltyEvent struct {
	CommittedBy struct {
		First Localized `json:"firstName"`
		Last  Localized `json:"lastName"`
	} `json:"committedByPlayer"`
	Time        string    `json:"timeInPeriod"`
	TeamAbbrev  Localized `json:"teamAbbrev"`
	Type        string    `json:"type"`
	Description string    `json:"descKey"`
	Duration    int       `json:"duration"`
}
type Star struct {
	Star       int       `json:"star"`
	PlayerID   int       `json:"playerId"`
	Name       Localized `json:"name"`
	TeamAbbrev string    `json:"teamAbbrev"`
	Position   string    `json:"position"`
}

func (g Game) Live() bool  { return g.State == "LIVE" || g.State == "CRIT" }
func (g Game) Final() bool { return g.State == "FINAL" || g.State == "OFF" || g.State == "OVER" }
func (g Game) Status() string {
	if g.ScheduleState == "PPD" || g.State == "PPD" {
		return "POSTPONED"
	}
	if g.ScheduleState == "CNCL" || g.ScheduleState == "DONT_PLAY" {
		return "CANCELLED"
	}
	if g.Final() {
		return "FINAL"
	}
	if g.Live() {
		p := fmt.Sprintf("%d%s", g.Period.Number, ordinal(g.Period.Number))
		if g.Period.Type == "OT" {
			p = "OT"
		}
		if g.Period.Type == "SO" {
			p = "SO"
		}
		if g.Clock.Intermission {
			return p + " INT"
		}
		if g.Clock.Remaining != "" {
			return p + " " + g.Clock.Remaining
		}
		return p
	}
	if !g.StartUTC.IsZero() {
		return g.StartUTC.In(Eastern()).Format("3:04 PM")
	}
	return "SCHEDULED"
}
func ordinal(n int) string {
	switch n {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	}
	return "th"
}
func Eastern() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.FixedZone("ET", -5*3600)
	}
	return loc
}
func (g Game) Score(t Team) string {
	if t.Score == nil {
		return ""
	}
	return fmt.Sprint(*t.Score)
}
func (g Game) Opponent(abbrev string) (string, string) {
	if g.Home.Abbrev == abbrev {
		return g.Away.Abbrev, "vs"
	}
	return g.Home.Abbrev, "@"
}

type Standing struct {
	TeamAbbrev         Localized `json:"teamAbbrev"`
	TeamName           Localized `json:"teamName"`
	Division           string    `json:"divisionName"`
	Conference         string    `json:"conferenceName"`
	DivisionSequence   int       `json:"divisionSequence"`
	ConferenceSequence int       `json:"conferenceSequence"`
	WildcardSequence   int       `json:"wildcardSequence"`
	LeagueSequence     int       `json:"leagueSequence"`
	GP                 int       `json:"gamesPlayed"`
	Wins               int       `json:"wins"`
	Losses             int       `json:"losses"`
	OTL                int       `json:"otLosses"`
	Points             int       `json:"points"`
	PointPct           float64   `json:"pointPctg"`
	RegulationWins     int       `json:"regulationWins"`
	ROW                int       `json:"regulationPlusOtWins"`
	GoalsFor           int       `json:"goalFor"`
	GoalsAgainst       int       `json:"goalAgainst"`
	GoalDiff           int       `json:"goalDifferential"`
	StreakCode         string    `json:"streakCode"`
	StreakCount        int       `json:"streakCount"`
}

func SortGames(g []Game) {
	sort.Slice(g, func(i, j int) bool { return g[i].StartUTC.Before(g[j].StartUTC) })
}
func StateTTL(g Game) time.Duration {
	if g.Live() {
		return 10 * time.Second
	}
	if g.Final() {
		return 5 * time.Minute
	}
	return 30 * time.Second
}

type RosterSpot struct {
	PlayerID int       `json:"playerId"`
	First    Localized `json:"firstName"`
	Last     Localized `json:"lastName"`
}
type PlayDetails struct {
	Scorer      int    `json:"scoringPlayerId"`
	Assist1     int    `json:"assist1PlayerId"`
	Assist2     int    `json:"assist2PlayerId"`
	CommittedBy int    `json:"committedByPlayerId"`
	Shooter     int    `json:"shootingPlayerId"`
	Winner      int    `json:"winningPlayerId"`
	Hitter      int    `json:"hittingPlayerId"`
	BlockedBy   int    `json:"blockingPlayerId"`
	PlayerID    int    `json:"playerId"`
	TeamID      int    `json:"eventOwnerTeamId"`
	AwayScore   *int   `json:"awayScore"`
	HomeScore   *int   `json:"homeScore"`
	Infraction  string `json:"descKey"`
	Minutes     int    `json:"duration"`
}
type Play struct {
	Type      string      `json:"typeDescKey"`
	Time      string      `json:"timeInPeriod"`
	Period    Period      `json:"periodDescriptor"`
	Situation string      `json:"situationCode"`
	Details   PlayDetails `json:"details"`
}
type PBP struct {
	Plays  []Play       `json:"plays"`
	Roster []RosterSpot `json:"rosterSpots"`
}

type RosterPlayer struct {
	ID       int       `json:"id"`
	First    Localized `json:"firstName"`
	Last     Localized `json:"lastName"`
	Number   int       `json:"sweaterNumber"`
	Position string    `json:"positionCode"`
	Shoots   string    `json:"shootsCatches"`
}

func (p RosterPlayer) FullName() string {
	return strings.TrimSpace(p.First.Default + " " + p.Last.Default)
}

type Roster struct {
	Forwards []RosterPlayer `json:"forwards"`
	Defense  []RosterPlayer `json:"defensemen"`
	Goalies  []RosterPlayer `json:"goalies"`
}

func (r Roster) Players() []RosterPlayer {
	all := make([]RosterPlayer, 0, len(r.Forwards)+len(r.Defense)+len(r.Goalies))
	all = append(all, r.Forwards...)
	all = append(all, r.Defense...)
	return append(all, r.Goalies...)
}

type ClubSkater struct {
	ID        int       `json:"playerId"`
	First     Localized `json:"firstName"`
	Last      Localized `json:"lastName"`
	Position  string    `json:"positionCode"`
	GP        int       `json:"gamesPlayed"`
	Goals     int       `json:"goals"`
	Assists   int       `json:"assists"`
	Points    int       `json:"points"`
	PlusMinus int       `json:"plusMinus"`
	Shots     int       `json:"shots"`
}

func (p ClubSkater) FullName() string {
	return strings.TrimSpace(p.First.Default + " " + p.Last.Default)
}

type ClubGoalie struct {
	ID      int       `json:"playerId"`
	First   Localized `json:"firstName"`
	Last    Localized `json:"lastName"`
	GP      int       `json:"gamesPlayed"`
	Wins    int       `json:"wins"`
	Losses  int       `json:"losses"`
	OTL     int       `json:"overtimeLosses"`
	GAA     float64   `json:"goalsAgainstAverage"`
	SavePct float64   `json:"savePercentage"`
}

func (p ClubGoalie) FullName() string {
	return strings.TrimSpace(p.First.Default + " " + p.Last.Default)
}

type ClubStats struct {
	Skaters []ClubSkater `json:"skaters"`
	Goalies []ClubGoalie `json:"goalies"`
}

// Injury is a current ESPN injury designation matched to an NHL club and player.
type Injury struct {
	Team   string
	Player string
	Status string
}

type PlayerProfile struct {
	ID           int       `json:"playerId"`
	First        Localized `json:"firstName"`
	Last         Localized `json:"lastName"`
	Number       int       `json:"sweaterNumber"`
	Position     string    `json:"position"`
	Team         string    `json:"currentTeamAbbrev"`
	BirthDate    string    `json:"birthDate"`
	BirthCity    Localized `json:"birthCity"`
	BirthCountry string    `json:"birthCountry"`
	Shoots       string    `json:"shootsCatches"`
	Featured     struct {
		Season  int `json:"season"`
		Regular struct {
			Season PlayerTotals `json:"subSeason"`
			Career PlayerTotals `json:"career"`
		} `json:"regularSeason"`
	} `json:"featuredStats"`
}

func (p PlayerProfile) FullName() string {
	return strings.TrimSpace(p.First.Default + " " + p.Last.Default)
}

type PlayerTotals struct {
	GP        int     `json:"gamesPlayed"`
	Goals     int     `json:"goals"`
	Assists   int     `json:"assists"`
	Points    int     `json:"points"`
	PlusMinus int     `json:"plusMinus"`
	Shots     int     `json:"shots"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	OTL       int     `json:"otLosses"`
	GAA       float64 `json:"goalsAgainstAvg"`
	SavePct   float64 `json:"savePctg"`
	Shutouts  int     `json:"shutouts"`
}
type PlayerGame struct {
	GameID       int     `json:"gameId"`
	Date         string  `json:"gameDate"`
	Opponent     string  `json:"opponentAbbrev"`
	HomeRoad     string  `json:"homeRoadFlag"`
	Goals        int     `json:"goals"`
	Assists      int     `json:"assists"`
	Points       int     `json:"points"`
	PlusMinus    int     `json:"plusMinus"`
	Shots        int     `json:"shots"`
	TOI          string  `json:"toi"`
	Decision     string  `json:"decision"`
	ShotsAgainst int     `json:"shotsAgainst"`
	GoalsAgainst int     `json:"goalsAgainst"`
	SavePct      float64 `json:"savePctg"`
}
type LeaderSkater struct {
	ID        int    `json:"playerId"`
	Name      string `json:"skaterFullName"`
	Team      string `json:"teamAbbrevs"`
	Position  string `json:"positionCode"`
	GP        int    `json:"gamesPlayed"`
	Goals     int    `json:"goals"`
	Assists   int    `json:"assists"`
	Points    int    `json:"points"`
	Shots     int    `json:"shots"`
	PlusMinus int    `json:"plusMinus"`
}
type LeaderGoalie struct {
	ID       int     `json:"playerId"`
	Name     string  `json:"goalieFullName"`
	Team     string  `json:"teamAbbrevs"`
	GP       int     `json:"gamesPlayed"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
	OTL      int     `json:"otLosses"`
	GAA      float64 `json:"goalsAgainstAverage"`
	SavePct  float64 `json:"savePct"`
	Shutouts int     `json:"shutouts"`
}

func (p PBP) Player(id int) string {
	for _, r := range p.Roster {
		if r.PlayerID == id {
			return strings.TrimSpace(r.First.Default + " " + r.Last.Default)
		}
	}
	return "Unknown player"
}
