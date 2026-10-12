package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Malvore/nhl-stats-vim/internal/nhl"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type fake struct {
	games     []nhl.Game
	standings []nhl.Standing
	injuries  []nhl.Injury
	err       error
}

func score(n int) *int { return &n }
func sampleGames() []nhl.Game {
	base := time.Date(2026, 10, 6, 23, 0, 0, 0, time.UTC)
	return []nhl.Game{
		{ID: 1, State: "LIVE", StartUTC: base, Away: nhl.Team{Abbrev: "PHI", Score: score(0)}, Home: nhl.Team{Abbrev: "NJD", Score: score(2)}, Period: nhl.Period{Number: 2}, Clock: nhl.Clock{Remaining: "13:29"}},
		{ID: 2, State: "FINAL", StartUTC: base.AddDate(0, 0, 1), Away: nhl.Team{Abbrev: "NJD", Score: score(2)}, Home: nhl.Team{Abbrev: "BOS", Score: score(3)}},
		{ID: 3, State: "FUT", StartUTC: base.AddDate(0, 0, 2), Away: nhl.Team{Abbrev: "NJD"}, Home: nhl.Team{Abbrev: "TOR"}},
	}
}
func plain(s string) string               { return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(s, "") }
func displayColumn(row, value string) int { return lipgloss.Width(row[:strings.Index(row, value)]) }
func TestVimMotions(t *testing.T) {
	games := sampleGames()
	m := model(games)
	m.today = games
	m.busy = false
	m.selected = 1
	m = key(m, "g")
	if m.selected != 1 {
		t.Fatal("single g moved selection")
	}
	m = key(m, "g")
	if m.selected != 0 {
		t.Fatal("gg did not select first game")
	}
	m = key(m, "G")
	if m.selected != 2 {
		t.Fatal("G did not select last game")
	}
	m = key(m, "gg")
	if m.selected != 0 {
		t.Fatal("coalesced gg did not select first game")
	}
	m = key(m, "G")
	m = key(m, "g")
	m = key(m, "j")
	m = key(m, "g")
	if m.selected != 2 {
		t.Fatal("interrupted g sequence moved selection")
	}
	m = key(m, "g")
	if m.selected != 0 {
		t.Fatal("fresh gg did not move")
	}
	m.screen = Team
	m.teamGames = games
	m.teamSelected = 1
	m = key(m, "G")
	if m.teamSelected != 2 {
		t.Fatal("team G")
	}
	m = key(key(m, "g"), "g")
	if m.teamSelected != 0 {
		t.Fatal("team gg")
	}
	m.screen = Standings
	m.standings = []nhl.Standing{{}, {}, {}}
	m.standingSelected = 1
	m = key(m, "G")
	if m.standingSelected != 2 {
		t.Fatal("standings G")
	}
	m = key(key(m, "g"), "g")
	if m.standingSelected != 0 {
		t.Fatal("standings gg")
	}
	m.screen = Game
	m.feed.SetContent(strings.Repeat("event\n", 40))
	m.feed.Height = 3
	m = key(m, "G")
	if m.feed.YOffset == 0 {
		t.Fatal("feed G")
	}
	m = key(key(m, "g"), "g")
	if m.feed.YOffset != 0 {
		t.Fatal("feed gg")
	}
	now := time.Date(2026, 10, 6, 20, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m.screen = Today
	m.selected = 2
	m = key(m, "g")
	now = now.Add(time.Second)
	m = key(m, "g")
	if m.selected != 2 {
		t.Fatal("stale g completed a motion")
	}
	m = key(m, "g")
	if m.selected != 0 {
		t.Fatal("new gg after timeout failed")
	}
}
func TestColumnsAndFooter(t *testing.T) {
	m := model(sampleGames())
	m.today = sampleGames()
	m.busy = false
	view := plain(m.View())
	if !strings.Contains(view, "PHI") || !strings.Contains(view, "j/k move") || !strings.Contains(view, "gg/G top/bottom") {
		t.Fatal(view)
	}
	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[len(lines)-1], "q quit") {
		t.Fatal("footer is not at bottom")
	}
	today := m.todayView()
	a, b, c := plain(today[2]), plain(today[3]), plain(today[4])
	if !strings.HasPrefix(a, "› ") || !strings.HasPrefix(b, "  ") {
		t.Fatalf("selected row cursor missing: %q / %q", a, b)
	}
	m.selected = 1
	rows := m.todayView()
	if !strings.HasPrefix(plain(rows[2]), "  ") || !strings.HasPrefix(plain(rows[3]), "› ") {
		t.Fatal("cursor did not move with selected row")
	}
	m = key(m, "j")
	rows = m.todayView()
	if !strings.HasPrefix(plain(rows[4]), "› ") {
		t.Fatal("j did not move the visible cursor immediately")
	}
	m = key(m, "k")
	rows = m.todayView()
	if !strings.HasPrefix(plain(rows[3]), "› ") {
		t.Fatal("k did not move the visible cursor immediately")
	}
	m.selected = 0
	if strings.Index(a, "PHI") >= strings.Index(a, "0") || strings.Index(a, "NJD") >= strings.Index(a, "2") || strings.Index(a, "2nd") <= strings.Index(a, "NJD") {
		t.Fatal(a)
	}
	if displayColumn(a, "2nd") != displayColumn(b, "FINAL") || displayColumn(a, "2nd") != displayColumn(c, "7:00 PM") {
		t.Fatalf("Today status drift: %q / %q / %q", a, b, c)
	}
	if displayColumn(a, "NJD") != displayColumn(b, "BOS") || displayColumn(a, "NJD") != displayColumn(c, "TOR") {
		t.Fatal("Today team drift")
	}
	m.screen = Team
	m.teamAbbr = "NJD"
	m.teamGames = sampleGames()
	team := m.teamView()
	a, b, c = plain(team[len(team)-3]), plain(team[len(team)-2]), plain(team[len(team)-1])
	if displayColumn(a, "PHI") != displayColumn(b, "BOS") || displayColumn(a, "PHI") != displayColumn(c, "TOR") {
		t.Fatalf("Team opponent drift: %q / %q / %q", a, b, c)
	}
	if strings.Index(a, "2–0") < 0 || strings.Index(b, "2–3") < 0 {
		t.Fatalf("team scores wrong: %q / %q", a, b)
	}
	if displayColumn(a, "2–0") != 26 || displayColumn(b, "2–3") != 26 {
		t.Fatalf("Team score drift: %q / %q", a, b)
	}
	if displayColumn(a, "2nd") != 45 || displayColumn(b, "FINAL") != 45 || displayColumn(c, "7:00 PM") != 45 {
		t.Fatalf("Team status/time drift: %q / %q / %q", a, b, c)
	}
	m.width = 200
	m.screen = Today
	today = m.todayView()
	a, b, c = plain(today[2]), plain(today[3]), plain(today[4])
	if displayColumn(a, "2nd") != 45 || displayColumn(b, "FINAL") != 45 || displayColumn(c, "7:00 PM") != 45 {
		t.Fatalf("wide Today spread across terminal: %q / %q / %q", a, b, c)
	}
	m.screen = Team
	team = m.teamView()
	a, b, c = plain(team[len(team)-3]), plain(team[len(team)-2]), plain(team[len(team)-1])
	if displayColumn(a, "2–0") != 26 || displayColumn(b, "2–3") != 26 || displayColumn(a, "2nd") != 47 || displayColumn(b, "FINAL") != 47 || displayColumn(c, "7:00 PM") != 47 {
		t.Fatalf("wide Team spread across terminal: %q / %q / %q", a, b, c)
	}
	m.screen = Game
	m.game = sampleGames()[0]
	m.feed.SetContent(strings.Repeat("event\n", 40))
	view = plain(m.View())
	lines = strings.Split(view, "\n")
	if len(lines) != 24 || !strings.Contains(lines[23], "q quit") {
		t.Fatal("game footer clipped")
	}
	m.width = 24
	m.height = 12
	m.screen = Today
	view = plain(m.View())
	if len(strings.Split(view, "\n")) != 12 {
		t.Fatal("narrow terminal height")
	}
}

func TestWelcomeHelpCenterAndTodayDateKeys(t *testing.T) {
	m := NewAt(fake{}, time.Now)
	view := plain(m.View())
	if !strings.Contains(view, "NHL Hockey in your terminal") {
		t.Fatal(view)
	}
	m = key(m, "?")
	lines := strings.Split(plain(m.View()), "\n")
	if len(lines) != m.height || strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("help is not vertically centered: %q", lines)
	}
	found := false
	for _, line := range lines {
		if strings.Contains(line, "NHL Stats Vim · Keys") {
			found = true
			if got := displayColumn(line, "NHL Stats Vim · Keys"); got < 25 {
				t.Fatalf("help heading is not horizontally centered: %q", line)
			}
		}
	}
	if !found {
		t.Fatal("help heading missing")
	}
	m = key(m, "?")
	m = special(m, tea.KeyEnter)
	footer := plain(m.View())
	if !strings.Contains(footer, "h/l date") || strings.Contains(footer, "[/] date") {
		t.Fatalf("Today footer has wrong date keys: %q", footer)
	}
}

func TestInjuryRosterAndPlayerOrdering(t *testing.T) {
	m := model(nil)
	m.screen, m.teamAbbr, m.teamTab, m.busy = Team, "NJD", 2, false
	m.teamRoster = nhl.Roster{
		Forwards: []nhl.RosterPlayer{
			{ID: 1, First: nhl.Localized{Default: "A"}, Last: nhl.Localized{Default: "Healthy"}, Position: "C"},
			{ID: 2, First: nhl.Localized{Default: "B"}, Last: nhl.Localized{Default: "Injured"}, Position: "L"},
		},
		Goalies: []nhl.RosterPlayer{
			{ID: 3, First: nhl.Localized{Default: "C"}, Last: nhl.Localized{Default: "Goalie"}, Position: "G"},
			{ID: 4, First: nhl.Localized{Default: "D"}, Last: nhl.Localized{Default: "Goalie"}, Position: "G"},
		},
	}
	m.teamStats = nhl.ClubStats{
		Skaters: []nhl.ClubSkater{{ID: 1, Points: 1}, {ID: 2, Points: 5}},
		Goalies: []nhl.ClubGoalie{{ID: 3, SavePct: .900}, {ID: 4, SavePct: .950}},
	}
	m.injuries = []nhl.Injury{{Team: "NJD", Player: "B Injured", Status: "DTD"}, {Team: "NJD", Player: "D Goalie", Status: "LTIR"}}
	view := plain(m.View())
	if !strings.Contains(view, "INJ") || !strings.Contains(view, "P/SV%") || !strings.Contains(view, "B Injured") || !strings.Contains(view, "DTD") || !strings.Contains(view, "LTIR") {
		t.Fatalf("roster injury columns missing: %q", view)
	}
	for _, pair := range [][2]string{{"B Injured", "A Healthy"}, {"D Goalie", "C Goalie"}} {
		if strings.Index(view, pair[0]) >= strings.Index(view, pair[1]) {
			t.Fatalf("roster not ordered by points/save percentage: %q", view)
		}
	}
	if id := m.teamSelectedPlayerID(); id != 2 {
		t.Fatalf("selected roster player = %d, want 2", id)
	}
	m.teamTab = 3
	if m.sortedTeamSkaters()[0].ID != 2 || m.sortedTeamGoalies()[0].ID != 4 || m.teamSelectedPlayerID() != 2 {
		t.Fatal("team stats and selected player use different order")
	}
	m.teamGoalies = true
	if m.teamSelectedPlayerID() != 4 {
		t.Fatal("goalie stats selection ignores save percentage order")
	}
	m.screen = Player
	m.player = nhl.PlayerProfile{ID: 2, Team: "NJD", First: nhl.Localized{Default: "B"}, Last: nhl.Localized{Default: "Injured"}}
	if !strings.Contains(plain(m.View()), "Injury: DTD") {
		t.Fatal("player injury missing")
	}
	m.player = nhl.PlayerProfile{ID: 1, Team: "NJD", First: nhl.Localized{Default: "A"}, Last: nhl.Localized{Default: "Healthy"}}
	if strings.Contains(plain(m.View()), "Injury:") {
		t.Fatal("healthy player displayed an injury")
	}
	m.injuries = append(m.injuries, nhl.Injury{Team: "NJD", Player: "Rémi Poirier", Status: "IR"})
	if got := m.injuryStatus("NJD", "Remi Poirier"); got != "IR" {
		t.Fatalf("accented injury name did not match: %q", got)
	}
}

func TestBoxScorePlayersSortByPointsAndSavePercentage(t *testing.T) {
	m := model(nil)
	m.screen, m.gameTab = Game, 1
	m.game.Away.Abbrev = "NJD"
	m.game.PlayerStats.Away.Forwards = []nhl.Skater{{ID: 1, Name: nhl.Localized{Default: "A Skater"}, Points: 1}, {ID: 2, Name: nhl.Localized{Default: "B Skater"}, Points: 3}}
	m.game.PlayerStats.Away.Goalies = []nhl.Goalie{{ID: 3, Name: nhl.Localized{Default: "C Goalie"}, Saves: 27, ShotsAgainst: 30}, {ID: 4, Name: nhl.Localized{Default: "D Goalie"}, Saves: 19, ShotsAgainst: 20}}
	m.rebuildFeed()
	if !reflect.DeepEqual(m.boxPlayerIDs, []int{2, 1, 4, 3}) || !strings.Contains(plain(m.feed.View()), "SV%") {
		t.Fatalf("box score order or save percentage missing: %v; %q", m.boxPlayerIDs, m.feed.View())
	}
}

func TestMetropolitanStandingsLabel(t *testing.T) {
	m := model(nil)
	m.screen = Standings
	m.standingView = standingsDivision
	m.busy = false
	m.standings = []nhl.Standing{
		{Division: "Atlantic", TeamAbbrev: nhl.Localized{Default: "OTT"}, GP: 2},
		{Division: "Metropolitan", TeamAbbrev: nhl.Localized{Default: "NJD"}, GP: 4},
	}
	rows := m.standingsView()
	atlantic, metro := plain(rows[5]), plain(rows[6])
	if !strings.HasPrefix(metro, "   METRO") || strings.Contains(metro, "METROPOLITA") {
		t.Fatal(metro)
	}
	if displayColumn(atlantic, "OTT") != displayColumn(metro, "NJD") {
		t.Fatalf("team column drift: %q / %q", atlantic, metro)
	}
}

func TestListSpacingKeepsSelectionAndFooterVisible(t *testing.T) {
	m := model(sampleGames())
	m.width, m.height = 100, 24
	m.screen = Standings
	m.busy = false
	for i := 0; i < 32; i++ {
		conference, division := "Eastern", "Atlantic"
		if i >= 16 {
			conference, division = "Western", "Central"
		}
		m.standings = append(m.standings, nhl.Standing{
			TeamAbbrev: nhl.Localized{Default: fmt.Sprintf("T%02d", i)},
			Conference: conference, Division: division,
			WildcardSequence: i%16 + 1,
		})
	}
	m.standingSelected = 31
	view := plain(m.View())
	lines := strings.Split(view, "\n")
	selectedVisible := false
	for _, line := range lines {
		if strings.HasPrefix(line, "›  ") && strings.Contains(line, "T31") {
			selectedVisible = true
		}
	}
	if len(lines) != m.height || !selectedVisible || !strings.Contains(lines[len(lines)-1], "q quit") {
		t.Fatalf("selection or footer clipped after spacing: %q", view)
	}
	if !strings.Contains(view, "STREAK\n\n") {
		t.Fatal(view)
	}
	m.screen = Team
	m.teamAbbr = "NJD"
	m.teamGames = sampleGames()
	view = plain(m.View())
	if !strings.Contains(view, "Recent / upcoming\n\n") || !strings.Contains(view, "›  Oct") {
		t.Fatalf("team schedule lacks section spacing: %q", view)
	}
}
func TestStandingsModesAndExpandedColumns(t *testing.T) {
	m := model(nil)
	m.screen = Standings
	m.busy = false
	m.standings = []nhl.Standing{
		{TeamAbbrev: nhl.Localized{Default: "NJD"}, Division: "Metropolitan", Conference: "Eastern", DivisionSequence: 2, ConferenceSequence: 4, LeagueSequence: 7, WildcardSequence: 1, GP: 4, Wins: 3, Points: 6, PointPct: .75, GoalsFor: 11, GoalsAgainst: 6},
		{TeamAbbrev: nhl.Localized{Default: "CAR"}, Division: "Metropolitan", Conference: "Eastern", DivisionSequence: 1, ConferenceSequence: 2, LeagueSequence: 3, GP: 4, Wins: 4, Points: 8},
	}
	if !strings.Contains(plain(m.View()), "P%") || !strings.Contains(plain(m.View()), "0.750") {
		t.Fatal("expanded standings columns missing")
	}
	if !strings.Contains(plain(m.standingsView()[1]), "[Wild Card]   Division   Conference   League") {
		t.Fatalf("standings options are not shown left to right: %q", plain(m.standingsView()[1]))
	}
	m = key(m, "l")
	if m.standingView != standingsDivision || !strings.Contains(plain(m.standingsView()[1]), "Wild Card   [Division]   Conference   League") {
		t.Fatal("l did not move the standings option right")
	}
	m = key(m, "h")
	if m.standingView != standingsWildCard || !strings.Contains(plain(m.standingsView()[1]), "[Wild Card]   Division   Conference   League") {
		t.Fatal("h did not move the standings option left")
	}
	for _, name := range []string{"Division", "Conference", "League", "Wild Card"} {
		m = special(m, tea.KeyTab)
		if !strings.Contains(plain(m.View()), "Standings · "+name) {
			t.Fatalf("missing %s standings view", name)
		}
	}
	m = special(m, tea.KeyTab)
	m = special(m, tea.KeyTab)
	m = special(m, tea.KeyTab)
	if m.sortedStandings()[0].TeamAbbrev.Default != "CAR" {
		t.Fatal("league standings order failed")
	}
}

func TestStandingsGroupTeamGapAt80Columns(t *testing.T) {
	m := model(nil)
	m.screen, m.busy, m.width = Standings, false, 80
	m.standings = []nhl.Standing{{
		TeamAbbrev: nhl.Localized{Default: "NJD"},
		Division:   "Metropolitan", Conference: "Eastern",
		DivisionSequence: 2, ConferenceSequence: 4, LeagueSequence: 7,
		WildcardSequence: 1, GoalsAgainst: 6,
	}}
	for mode := standingsWildCard; mode <= standingsLeague; mode++ {
		m.standingView = mode
		lines := m.standingsView()
		header := plain(lines[3])
		if got := displayColumn(header, "TEAM"); got != 17 {
			t.Fatalf("mode %d TEAM header at %d, want 17", mode, got)
		}
		found := false
		for _, line := range lines[4:] {
			row := plain(line)
			if !strings.Contains(row, "NJD") {
				continue
			}
			found = true
			if got := displayColumn(row, "NJD"); got != 17 {
				t.Errorf("mode %d team at %d, want 17", mode, got)
			}
			if lipgloss.Width(row) > m.width {
				t.Errorf("mode %d row exceeds 80 columns: %q", mode, row)
			}
			if mode == standingsWildCard && !strings.Contains(row, "WILD CARD 1   NJD") {
				t.Errorf("Wild Card place/team gap is not three spaces: %q", row)
			}
		}
		if !found {
			t.Fatalf("mode %d is missing NJD", mode)
		}
	}
}

func TestWildCardStandingsLabelOnlyTwoSpotsPerConference(t *testing.T) {
	m := model(nil)
	m.screen = Standings
	m.busy = false
	m.width = 120
	m.height = 50
	m.standingView = standingsWildCard
	m.standings = []nhl.Standing{
		{TeamAbbrev: nhl.Localized{Default: "BOS"}, Division: "Atlantic", Conference: "Eastern", DivisionSequence: 1},
		{TeamAbbrev: nhl.Localized{Default: "NYR"}, Division: "Metropolitan", Conference: "Eastern", DivisionSequence: 1},
		{TeamAbbrev: nhl.Localized{Default: "BUF"}, Division: "Atlantic", Conference: "Eastern", WildcardSequence: 1},
		{TeamAbbrev: nhl.Localized{Default: "TOR"}, Division: "Atlantic", Conference: "Eastern", WildcardSequence: 2},
		{TeamAbbrev: nhl.Localized{Default: "MTL"}, Division: "Atlantic", Conference: "Eastern", WildcardSequence: 3},
		{TeamAbbrev: nhl.Localized{Default: "COL"}, Division: "Central", Conference: "Western", DivisionSequence: 1},
		{TeamAbbrev: nhl.Localized{Default: "VGK"}, Division: "Pacific", Conference: "Western", DivisionSequence: 1},
		{TeamAbbrev: nhl.Localized{Default: "SJS"}, Division: "Pacific", Conference: "Western", WildcardSequence: 1},
		{TeamAbbrev: nhl.Localized{Default: "SEA"}, Division: "Pacific", Conference: "Western", WildcardSequence: 2},
		{TeamAbbrev: nhl.Localized{Default: "NSH"}, Division: "Central", Conference: "Western", WildcardSequence: 3},
	}

	var labeled []string
	view := m.standingsView()
	if !strings.Contains(strings.Join(view, "\n"), "EASTERN CONFERENCE") || !strings.Contains(strings.Join(view, "\n"), "WESTERN CONFERENCE") {
		t.Fatalf("Wild Card view is missing conference separators: %q", view)
	}
	if !strings.Contains(plain(view[3]), "PLACE") {
		t.Fatalf("Wild Card column is not identified as PLACE: %q", plain(view[3]))
	}
	for _, line := range view[4:] {
		row := plain(line)
		if strings.Contains(row, "WILD CARD") {
			labeled = append(labeled, row)
		}
	}
	if len(labeled) != 4 {
		t.Fatalf("want two wild-card labels per conference, got %d: %q", len(labeled), labeled)
	}
	places := map[string]string{
		"BOS": "ATL 1", "NYR": "MET 1", "COL": "CEN 1", "VGK": "PAC 1",
		"BUF": "WILD CARD 1", "TOR": "WILD CARD 2", "SJS": "WILD CARD 1", "SEA": "WILD CARD 2",
		"MTL": "", "NSH": "",
	}
	for team, want := range places {
		found := false
		for _, row := range view[4:] {
			plainRow := plain(row)
			if !strings.Contains(plainRow, team) || displayColumn(plainRow, team) != 17 {
				continue
			}
			found = true
			if strings.HasPrefix(plainRow, "›  ") {
				plainRow = "   " + strings.TrimPrefix(plainRow, "›  ")
			}
			if got := strings.TrimSpace(plainRow[3:16]); got != want {
				t.Errorf("%s place = %q, want %q", team, got, want)
			}
		}
		if !found {
			t.Errorf("team %s missing from Wild Card view", team)
		}
	}
}

func (f fake) Today(context.Context, string, bool) ([]nhl.Game, error) { return f.games, f.err }
func (f fake) Boxscore(_ context.Context, _ int, _ time.Duration, _ bool) (nhl.Game, error) {
	return f.games[0], f.err
}
func (f fake) Plays(context.Context, int, time.Duration, bool) (nhl.PBP, error) {
	return nhl.PBP{}, f.err
}
func (f fake) Summary(context.Context, int, time.Duration, bool) (nhl.GameSummary, error) {
	return nhl.GameSummary{}, f.err
}
func (f fake) Standings(context.Context, string, bool) ([]nhl.Standing, error) {
	return f.standings, f.err
}
func (f fake) TeamSchedule(context.Context, string, int, bool) ([]nhl.Game, error) {
	return f.games, f.err
}
func (f fake) Roster(context.Context, string, int, bool) (nhl.Roster, error) {
	return nhl.Roster{}, f.err
}
func (f fake) ClubStats(context.Context, string, int, bool) (nhl.ClubStats, error) {
	return nhl.ClubStats{}, f.err
}
func (f fake) Injuries(context.Context, bool) ([]nhl.Injury, error) { return f.injuries, f.err }
func (f fake) Player(context.Context, int, bool) (nhl.PlayerProfile, error) {
	return nhl.PlayerProfile{}, f.err
}
func (f fake) PlayerLog(context.Context, int, int, bool) ([]nhl.PlayerGame, error) { return nil, f.err }
func (f fake) Leaders(context.Context, int, bool, int, bool) ([]nhl.LeaderSkater, []nhl.LeaderGoalie, error) {
	return nil, nil, f.err
}
func model(games []nhl.Game) Model {
	f := fake{games: games, standings: []nhl.Standing{{TeamAbbrev: nhl.Localized{Default: "NJD"}, TeamName: nhl.Localized{Default: "New Jersey Devils"}}}}
	m := NewAt(f, func() time.Time { return time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC) })
	return special(m, tea.KeyEnter)
}
func key(m Model, k string) Model {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return next.(Model)
}
func special(m Model, k tea.KeyType) Model {
	next, _ := m.Update(tea.KeyMsg{Type: k})
	return next.(Model)
}
func TestWelcomeEntryAndShortcuts(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 6, 20, 0, 0, 0, time.UTC) }
	m := NewAt(fake{games: sampleGames()}, now)
	if m.screen != Welcome || m.busy || m.Init() != nil {
		t.Fatal("welcome should wait for input")
	}
	view := plain(m.View())
	if !strings.Contains(view, "Hockey in your terminal") || !strings.Contains(view, "Press enter for today's games") || !strings.Contains(view, "enter Today") {
		t.Fatal(view)
	}
	if !strings.Contains(strings.Split(view, "\n")[23], "q quit") {
		t.Fatal("welcome footer missing")
	}
	m = key(m, "r")
	if m.screen != Welcome || m.busy {
		t.Fatal("refresh should not load on welcome")
	}
	m = key(m, "?")
	if !strings.Contains(m.View(), "Vim-style") || strings.Contains(plain(m.View()), "d  Devils") {
		t.Fatal("welcome help missing")
	}
	m = key(m, "?")
	m = key(m, "s")
	if m.screen != Standings || !m.busy {
		t.Fatal("standings shortcut from welcome")
	}
	m.standings = []nhl.Standing{{TeamAbbrev: nhl.Localized{Default: "NJD"}}}
	m = special(m, tea.KeyEnter)
	if m.screen != Team {
		t.Fatal("team did not open from standings")
	}
	m.teamGames = sampleGames()
	m = special(m, tea.KeyEnter)
	if m.screen != Game {
		t.Fatal("game did not open from team")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Team {
		t.Fatal("game did not return to team")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Standings {
		t.Fatal("team did not return to standings")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Welcome || m.busy {
		t.Fatal("standings did not return to welcome")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = next.(Model)
	if m.screen != Welcome || m.busy || cmd != nil || strings.Contains(plain(m.View()), "d Devils") {
		t.Fatal("removed shortcut still acts on welcome")
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.screen != Today || !m.busy || cmd == nil {
		t.Fatal("enter did not load Today")
	}
	resultMsg := cmd()
	if resultMsg.(result).kind != Today {
		t.Fatal("wrong initial load")
	}
	next, _ = m.Update(resultMsg)
	m = next.(Model)
	if m.busy || len(m.today) != len(sampleGames()) {
		t.Fatal("Today failed to load")
	}
}
func TestWelcomeNarrow(t *testing.T) {
	m := NewAt(fake{}, time.Now)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 24, Height: 12})
	m = next.(Model)
	view := plain(m.View())
	lines := strings.Split(view, "\n")
	if len(lines) != 12 || !strings.Contains(view, "Enter for Today") || !strings.Contains(lines[11], "q quit") {
		t.Fatal(view)
	}
	for _, line := range lines {
		if lipgloss.Width(line) > 24 {
			t.Fatalf("welcome overflows narrow terminal: %q", line)
		}
	}
}
func TestNavigation(t *testing.T) {
	g := nhl.Game{ID: 12, State: "LIVE", Away: nhl.Team{Abbrev: "NJD"}, Home: nhl.Team{Abbrev: "NYR"}}
	m := model([]nhl.Game{g})
	if strings.Contains(m.View(), "No games today") {
		t.Fatal("loading screen showed false empty state")
	}
	if m.screen != Today {
		t.Fatal("not Today")
	}
	next, _ := m.Update(result{kind: Today, games: []nhl.Game{g}})
	m = next.(Model)
	m = special(m, tea.KeyEnter)
	if m.screen != Game {
		t.Fatal("game not opened")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Today {
		t.Fatal("back failed")
	}
	m = key(m, "s")
	if m.screen != Standings {
		t.Fatal("standings not opened")
	}
	next, _ = m.Update(result{kind: Standings, standings: []nhl.Standing{{TeamAbbrev: nhl.Localized{Default: "NJD"}}}})
	m = next.(Model)
	m = special(m, tea.KeyTab)
	if m.standingView != 1 {
		t.Fatal("tab failed")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = next.(Model)
	if m.screen != Standings || cmd != nil || strings.Contains(plain(m.View()), "d Devils") {
		t.Fatal("removed shortcut still acts on standings")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	if m.width != 80 || m.feed.Width != 76 {
		t.Fatal("resize")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("quit command missing")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("quit failed")
	}
}
func TestEmptyAndFailure(t *testing.T) {
	m := model(nil)
	next, _ := m.Update(result{kind: Today, games: []nhl.Game{}})
	m = next.(Model)
	if !strings.Contains(m.View(), "No games today") {
		t.Fatal(m.View())
	}
	next, _ = m.Update(result{kind: Today, err: errors.New("offline")})
	m = next.(Model)
	if !strings.Contains(m.View(), "NHL API unavailable") {
		t.Fatal(m.View())
	}
}

func TestDateBrowsingAndStaleResult(t *testing.T) {
	m := model(sampleGames())
	m.today = sampleGames()
	m.busy = false
	m.selected = 2
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m = next.(Model)
	if m.day() != "2026-10-06" || m.selected != 0 || !m.busy || cmd == nil {
		t.Fatal("next date did not load")
	}
	next, _ = m.Update(result{kind: Today, day: "2026-10-05", games: sampleGames()})
	m = next.(Model)
	if len(m.today) != 0 {
		t.Fatal("stale date replaced current scores")
	}
	if got := cmd().(result).day; got != "2026-10-06" {
		t.Fatal(got)
	}
	m = key(m, "[")
	if m.day() != "2026-10-05" {
		t.Fatal("previous date failed")
	}
	m = key(m, "]")
	m = key(m, "t")
	if m.day() != "2026-10-05" {
		t.Fatal("today shortcut failed")
	}
}

func TestGamecenterTabsAndEventFilter(t *testing.T) {
	m := model(sampleGames())
	m.screen = Game
	m.game = sampleGames()[0]
	if err := json.Unmarshal([]byte(`{"summary":{"scoring":[{"periodDescriptor":{"number":1},"goals":[{"name":{"default":"A. Scorer"},"teamAbbrev":{"default":"NJD"},"timeInPeriod":"04:12"}]}],"threeStars":[{"star":1,"name":{"default":"A. Scorer"},"teamAbbrev":"NJD"}]}}`), &m.summary); err != nil {
		t.Fatal(err)
	}
	m.pbp = nhl.PBP{Plays: []nhl.Play{{Type: "goal", Time: "04:12", Details: nhl.PlayDetails{Scorer: 7}}, {Type: "faceoff", Time: "04:13", Details: nhl.PlayDetails{Winner: 8}}}, Roster: []nhl.RosterSpot{{PlayerID: 7, Last: nhl.Localized{Default: "Scorer"}}, {PlayerID: 8, Last: nhl.Localized{Default: "Winner"}}}}
	m.game.PlayerStats.Away.Forwards = []nhl.Skater{{Name: nhl.Localized{Default: "A. Scorer"}, Goals: 1, Points: 1}}
	m.rebuildFeed()
	if !strings.Contains(plain(m.View()), "THREE STARS") {
		t.Fatal("summary tab missing")
	}
	m = special(m, tea.KeyTab)
	if m.gameTab != 1 || !strings.Contains(plain(m.View()), "SKATERS") {
		t.Fatal("box-score tab missing")
	}
	m = special(m, tea.KeyTab)
	if m.gameTab != 2 || !strings.Contains(plain(m.View()), "FACEOFF") {
		t.Fatal("full event feed missing")
	}
	m = key(m, "f")
	if !m.keyEventsOnly || strings.Contains(plain(m.View()), "FACEOFF") || !strings.Contains(plain(m.View()), "GOAL") {
		t.Fatal("event filter failed")
	}
	m = key(m, "h")
	if m.gameTab != 1 {
		t.Fatal("h did not switch previous game tab")
	}
}

func TestTeamPlayerAndLeadersNavigation(t *testing.T) {
	m := model(sampleGames())
	m = key(m, "a")
	if m.screen != Teams || !m.busy {
		t.Fatal("all-team browser did not open")
	}
	m.standings = []nhl.Standing{{TeamAbbrev: nhl.Localized{Default: "NJD"}, TeamName: nhl.Localized{Default: "New Jersey Devils"}}}
	m.busy = false
	if !strings.Contains(plain(m.View()), "New Jersey Devils") {
		t.Fatal("team browser did not render")
	}
	m = special(m, tea.KeyEnter)
	if m.screen != Team || m.teamAbbr != "NJD" {
		t.Fatal("team browser drill-down failed")
	}
	m.busy = false
	m.teamAllGames = sampleGames()
	m.teamGames = sampleGames()
	m.teamRoster = nhl.Roster{Forwards: []nhl.RosterPlayer{{ID: 7, First: nhl.Localized{Default: "A."}, Last: nhl.Localized{Default: "Skater"}}}}
	m.teamStats = nhl.ClubStats{Skaters: []nhl.ClubSkater{{ID: 7, First: nhl.Localized{Default: "A."}, Last: nhl.Localized{Default: "Skater"}, Points: 2}}, Goalies: []nhl.ClubGoalie{{ID: 8, First: nhl.Localized{Default: "B."}, Last: nhl.Localized{Default: "Goalie"}, SavePct: .925}}}
	m = special(m, tea.KeyTab)
	if m.teamTab != 1 || !strings.Contains(plain(m.View()), "Season schedule") {
		t.Fatal("full schedule tab failed")
	}
	m = special(m, tea.KeyTab)
	if m.teamTab != 2 || !strings.Contains(plain(m.View()), "A. Skater") {
		t.Fatal("roster tab failed")
	}
	m = special(m, tea.KeyEnter)
	if m.screen != Player || m.playerID != 7 {
		t.Fatal("roster player drill-down failed")
	}
	m.player = nhl.PlayerProfile{ID: 7, First: nhl.Localized{Default: "A."}, Last: nhl.Localized{Default: "Skater"}}
	m.playerLog = []nhl.PlayerGame{{GameID: 42, Date: "2026-10-05", Opponent: "NYR", Points: 2}}
	m.busy = false
	if !strings.Contains(plain(m.View()), "Game log") {
		t.Fatal("player log missing")
	}
	m = special(m, tea.KeyEnter)
	if m.screen != Game || m.game.ID != 42 {
		t.Fatal("player game drill-down failed")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Player {
		t.Fatal("game did not return to player")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Team {
		t.Fatal("player did not return to team")
	}
	m = special(m, tea.KeyTab)
	if m.teamTab != 3 || !strings.Contains(plain(m.View()), "[Skaters]   Goalies") {
		t.Fatal("stats tab failed")
	}
	m = key(m, "f")
	if !m.teamGoalies || !strings.Contains(plain(m.View()), "Skaters   [Goalies]") {
		t.Fatal("goalie stats toggle failed")
	}
	m = key(m, "L")
	if m.screen != Leaders {
		t.Fatal("leaders did not open")
	}
	m.busy = false
	m.skaterLeaders = []nhl.LeaderSkater{{ID: 7, Name: "A. Skater", Points: 2}}
	if !strings.Contains(plain(m.View()), "A. Skater") {
		t.Fatal("skater leaders missing")
	}
	if !strings.Contains(plain(m.leadersView()[1]), "[Skaters]   Goalies") || !strings.Contains(plain(m.leadersView()[2]), "[Points]   Goals   Assists   Shots") {
		t.Fatal("leader group and metric options are not visible")
	}
	m = key(m, "m")
	if m.leadersMetric != 1 || !strings.Contains(plain(m.leadersView()[2]), "Points   [Goals]") {
		t.Fatal("leader metric did not change")
	}
	m = key(m, "l")
	if !m.leadersGoalies || m.leadersMetric != 0 {
		t.Fatal("l did not move to the goalie option")
	}
	m = key(m, "h")
	if m.leadersGoalies || m.leadersMetric != 0 {
		t.Fatal("h did not move to the skater option")
	}
	m = special(m, tea.KeyTab)
	if m.leadersFocus != 1 || m.leadersGoalies {
		t.Fatal("tab did not focus the metric options")
	}
	m = key(m, "l")
	if m.leadersMetric != 1 || !strings.Contains(plain(m.leadersView()[2]), "Points   [Goals]") {
		t.Fatal("l did not change the focused metric")
	}
	m = special(m, tea.KeyTab)
	m = key(m, "l")
	if !m.leadersGoalies || m.leadersMetric != 0 || !strings.Contains(plain(m.leadersView()[1]), "Skaters   [Goalies]") || !strings.Contains(plain(m.leadersView()[2]), "[Save %]   Wins   GAA") {
		t.Fatal("h/l did not change the focused group")
	}
	m.goalieLeaders = []nhl.LeaderGoalie{{ID: 8, Name: "B. Goalie", SavePct: .925}}
	m.busy = false
	if !strings.Contains(plain(m.View()), "B. Goalie") {
		t.Fatal("goalie leaders missing")
	}
	m = special(m, tea.KeyEnter)
	if m.screen != Player || m.playerID != 8 {
		t.Fatal("leader to player drill-down failed")
	}
}

func TestExpandedPanelsFitNarrowTerminal(t *testing.T) {
	m := model(sampleGames())
	m.today = sampleGames()
	m.teamAbbr = "NJD"
	m.teamGames = sampleGames()
	m.teamAllGames = sampleGames()
	m.standings = []nhl.Standing{{TeamAbbrev: nhl.Localized{Default: "NJD"}, TeamName: nhl.Localized{Default: "New Jersey Devils"}}}
	m.skaterLeaders = []nhl.LeaderSkater{{Name: "A very long hockey player name that would overlap columns", Team: "NJD"}}
	m.game = sampleGames()[0]
	m.height, m.width = 12, 24
	for _, screen := range []Screen{Today, Game, Standings, Team, Player, Teams, Leaders} {
		m.screen = screen
		m.busy = false
		view := plain(m.View())
		lines := strings.Split(view, "\n")
		if len(lines) != 12 {
			t.Fatalf("screen %d height %d", screen, len(lines))
		}
		for _, line := range lines {
			if lipgloss.Width(line) > 24 {
				t.Fatalf("screen %d overflows: %q", screen, line)
			}
		}
	}
	m.help = true
	for _, line := range strings.Split(plain(m.View()), "\n") {
		if lipgloss.Width(line) > 24 {
			t.Fatalf("help overflows: %q", line)
		}
	}
}
func TestGoalieProfileUsesGoalieStatistics(t *testing.T) {
	m := model(nil)
	m.screen = Player
	m.busy = false
	m.player = nhl.PlayerProfile{ID: 8, Position: "G", First: nhl.Localized{Default: "B."}, Last: nhl.Localized{Default: "Goalie"}}
	m.player.Featured.Season = 20262027
	m.player.Featured.Regular.Season = nhl.PlayerTotals{GP: 2, Wins: 1, Losses: 0, GAA: 1.89, SavePct: .92}
	m.playerLog = []nhl.PlayerGame{{Date: "2026-10-01", Opponent: "PHI", Decision: "W", ShotsAgainst: 25, GoalsAgainst: 2, SavePct: .92, TOI: "63:23"}}
	view := plain(m.View())
	if !strings.Contains(view, "2026-27") || !strings.Contains(view, "SV%") || !strings.Contains(view, "0.920") || !strings.Contains(view, "63:23") {
		t.Fatal(view)
	}
}
func TestPlayerGameLogOpponentFits(t *testing.T) {
	m := model(nil)
	m.screen = Player
	m.busy = false
	m.player = nhl.PlayerProfile{ID: 7, Position: "C", First: nhl.Localized{Default: "A."}, Last: nhl.Localized{Default: "Skater"}}
	m.playerLog = []nhl.PlayerGame{{Date: "2026-10-03", Opponent: "CAR", HomeRoad: "H", Goals: 1, TOI: "16:08"}}
	view := plain(m.View())
	if !strings.Contains(view, "vs CAR") || strings.Contains(view, "vs C…") {
		t.Fatal(view)
	}
}
func TestRecentScheduleKeepsRegularSeason(t *testing.T) {
	m := model(nil)
	games := sampleGames()
	games[0].GameType = 1
	games[1].GameType = 2
	games[2].GameType = 2
	m.now = func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) }
	rows := m.teamRows(games)
	if len(rows) != 2 || rows[0].ID != 2 || rows[1].ID != 3 {
		t.Fatalf("recent regular-season games: %+v", rows)
	}
}
func TestStaleTeamLoadDoesNotReplaceCurrentTeam(t *testing.T) {
	m := model(nil)
	m.screen = Team
	m.teamAbbr = "NJD"
	m.busy = true
	next, _ := m.Update(result{kind: Team, teamAbbr: "NYR", games: sampleGames()})
	m = next.(Model)
	if !m.busy || len(m.teamGames) != 0 {
		t.Fatal("stale team response was applied")
	}
}
func TestBackRestoresNestedContext(t *testing.T) {
	m := model(sampleGames())
	m.today = sampleGames()
	m.busy = false
	m.selected = 1
	m = special(m, tea.KeyEnter)
	m.busy = false
	m = key(m, "a")
	m.busy = false
	m.standings = []nhl.Standing{{TeamAbbrev: nhl.Localized{Default: "NJD"}, TeamName: nhl.Localized{Default: "New Jersey Devils"}}}
	m = special(m, tea.KeyEnter)
	m.teamTab = 2
	m.busy = false
	m = special(m, tea.KeyEsc)
	if m.screen != Teams {
		t.Fatal("team did not return to team browser")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Game || m.game.ID != 2 {
		t.Fatal("team browser did not restore original game")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Today || m.selected != 1 {
		t.Fatal("game did not restore scores selection")
	}
}
func TestBoxScorePlayerSelection(t *testing.T) {
	m := model(nil)
	m.screen = Game
	m.busy = false
	m.game = sampleGames()[0]
	m.game.PlayerStats.Away.Forwards = []nhl.Skater{{ID: 7, Name: nhl.Localized{Default: "A. Skater"}}, {ID: 8, Name: nhl.Localized{Default: "B. Skater"}}}
	m.gameTab = 1
	m.rebuildFeed()
	if len(m.boxPlayerIDs) != 2 || !strings.Contains(plain(m.View()), "enter player") {
		t.Fatal("box-score player rows missing")
	}
	m = key(m, "j")
	if m.boxSelected != 1 {
		t.Fatal("box-score selection did not move")
	}
	m = key(m, "G")
	if m.boxSelected != 1 {
		t.Fatal("box-score G failed")
	}
	m = key(key(m, "g"), "g")
	if m.boxSelected != 0 {
		t.Fatal("box-score gg failed")
	}
	m = key(m, "j")
	m = special(m, tea.KeyEnter)
	if m.screen != Player || m.playerID != 8 {
		t.Fatal("box-score player did not open")
	}
	m = special(m, tea.KeyEsc)
	if m.screen != Game || m.gameTab != 1 || m.boxSelected != 1 {
		t.Fatal("box-score selection was not restored")
	}
}
