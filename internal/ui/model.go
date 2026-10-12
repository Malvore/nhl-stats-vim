package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Malvore/nhl-stats-vim/internal/nhl"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/unicode/norm"
)

type Source interface {
	Today(context.Context, string, bool) ([]nhl.Game, error)
	Boxscore(context.Context, int, time.Duration, bool) (nhl.Game, error)
	Plays(context.Context, int, time.Duration, bool) (nhl.PBP, error)
	Summary(context.Context, int, time.Duration, bool) (nhl.GameSummary, error)
	Standings(context.Context, string, bool) ([]nhl.Standing, error)
	TeamSchedule(context.Context, string, int, bool) ([]nhl.Game, error)
	Roster(context.Context, string, int, bool) (nhl.Roster, error)
	ClubStats(context.Context, string, int, bool) (nhl.ClubStats, error)
	Injuries(context.Context, bool) ([]nhl.Injury, error)
	Player(context.Context, int, bool) (nhl.PlayerProfile, error)
	PlayerLog(context.Context, int, int, bool) ([]nhl.PlayerGame, error)
	Leaders(context.Context, int, bool, int, bool) ([]nhl.LeaderSkater, []nhl.LeaderGoalie, error)
}
type Screen int

const (
	Welcome Screen = iota
	Today
	Game
	Standings
	Team
	Player
	Teams
	Leaders
)

const (
	standingsWildCard = iota
	standingsDivision
	standingsConference
	standingsLeague
)

type result struct {
	generation     uint64
	detail         string
	force          bool
	kind           Screen
	day            string
	games          []nhl.Game
	game           nhl.Game
	plays          nhl.PBP
	summary        nhl.GameSummary
	standings      []nhl.Standing
	roster         nhl.Roster
	stats          nhl.ClubStats
	injuries       []nhl.Injury
	player         nhl.PlayerProfile
	playerLog      []nhl.PlayerGame
	skaterLeaders  []nhl.LeaderSkater
	goalieLeaders  []nhl.LeaderGoalie
	err            error
	target         int
	leadersGoalies bool
	leadersMetric  int
	teamAbbr       string
}
type tick struct{ generation uint64 }
type navigationState struct {
	screen                                       Screen
	viewDay                                      time.Time
	today                                        []nhl.Game
	selected                                     int
	game                                         nhl.Game
	pbp                                          nhl.PBP
	summary                                      nhl.GameSummary
	gameTab, boxSelected                         int
	keyEventsOnly                                bool
	feedOffset                                   int
	standings                                    []nhl.Standing
	standingView, standingSelected               int
	teamAbbr                                     string
	teamGames, teamAllGames                      []nhl.Game
	teamRoster                                   nhl.Roster
	teamStats                                    nhl.ClubStats
	injuries                                     []nhl.Injury
	injuryUpdated                                time.Time
	injuryAvailable                              bool
	teamTab, teamStatsFocus, teamSelected        int
	teamGoalies                                  bool
	playerID                                     int
	player                                       nhl.PlayerProfile
	playerLog                                    []nhl.PlayerGame
	playerSelected, teamsSelected                int
	leadersGoalies, goalieQualified              bool
	leadersMetric, leadersFocus, leadersSelected int
	skaterLeaders                                []nhl.LeaderSkater
	goalieLeaders                                []nhl.LeaderGoalie
	err                                          error
	warnings                                     []string
}
type Model struct {
	source           Source
	ctx              context.Context
	now              func() time.Time
	screen           Screen
	previous         Screen
	standingsBack    Screen
	teamBack         Screen
	help             bool
	width, height    int
	busy             bool
	err              error
	warnings         []string
	generation       uint64
	pending          map[string]bool
	detailErrors     map[string]string
	injuryUpdated    time.Time
	injuryAvailable  bool
	today            []nhl.Game
	viewDay          time.Time
	selected         int
	game             nhl.Game
	pbp              nhl.PBP
	summary          nhl.GameSummary
	gameTab          int
	boxSelected      int
	boxPlayerIDs     []int
	boxLineOffsets   []int
	keyEventsOnly    bool
	feed             viewport.Model
	standings        []nhl.Standing
	standingView     int
	standingSelected int
	teamAbbr         string
	teamGames        []nhl.Game
	teamAllGames     []nhl.Game
	teamRoster       nhl.Roster
	teamStats        nhl.ClubStats
	injuries         []nhl.Injury
	teamTab          int
	teamGoalies      bool
	teamSelected     int
	playerBack       Screen
	playerID         int
	player           nhl.PlayerProfile
	playerLog        []nhl.PlayerGame
	playerSelected   int
	teamsBack        Screen
	teamsSelected    int
	leadersBack      Screen
	leadersGoalies   bool
	leadersMetric    int
	leadersFocus     int
	teamStatsFocus   int
	goalieQualified  bool
	leadersSelected  int
	skaterLeaders    []nhl.LeaderSkater
	goalieLeaders    []nhl.LeaderGoalie
	gPending         time.Time
	backStack        []navigationState
}

func New(source Source) Model { return NewWithContext(context.Background(), source) }
func NewWithContext(ctx context.Context, source Source) Model {
	m := NewAt(source, time.Now)
	m.ctx = ctx
	return m
}
func NewAt(source Source, now func() time.Time) Model {
	v := viewport.New(78, 15)
	return Model{source: source, ctx: context.Background(), now: now, viewDay: now().In(nhl.Eastern()), screen: Welcome, width: 80, height: 24, feed: v}
}
func (m Model) Init() tea.Cmd { return nil }
func (m Model) day() string   { return m.viewDay.Format("2006-01-02") }
func (m *Model) beginLoad(kind Screen, force bool) tea.Cmd {
	m.generation++
	m.pending = nil
	return m.load(kind, force)
}
func (m Model) load(kind Screen, force bool) tea.Cmd {
	src := m.source
	parent := m.ctx
	day := m.day()
	standingsDay := m.now().In(nhl.Eastern()).Format("2006-01-02")
	g := m.game
	abbr := m.teamAbbr
	playerID := m.playerID
	goalies, metric := m.leadersGoalies, m.leadersMetric
	season := nhl.Season(m.now())
	generation := m.generation
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, 12*time.Second)
		defer cancel()
		r := result{kind: kind, target: g.ID, day: day, leadersGoalies: goalies, leadersMetric: metric, teamAbbr: abbr, generation: generation, force: force}
		if kind == Player {
			r.target = playerID
		}
		switch kind {
		case Today:
			r.games, r.err = src.Today(ctx, day, force)
		case Game:
			if g.ID == 0 {
				return r
			}
			ttl := nhl.StateTTL(g)
			r.game, r.err = src.Boxscore(ctx, g.ID, ttl, force)
		case Standings:
			r.standings, r.err = src.Standings(ctx, standingsDay, force)
		case Teams:
			r.standings, r.err = src.Standings(ctx, standingsDay, force)
		case Team:
			r.games, r.err = src.TeamSchedule(ctx, abbr, season, force)
		case Player:
			r.player, r.err = src.Player(ctx, playerID, force)
		case Leaders:
			r.skaterLeaders, r.goalieLeaders, r.err = src.Leaders(ctx, season, goalies, metric, force)
		}
		return r
	}
}
func (m Model) loadDetail(detail string, force bool) tea.Cmd {
	src, parent := m.source, m.ctx
	game, abbr, playerID := m.game, m.teamAbbr, m.playerID
	season := nhl.Season(m.now())
	standingsDay := m.now().In(nhl.Eastern()).Format("2006-01-02")
	r := result{kind: m.screen, generation: m.generation, detail: detail, target: game.ID, teamAbbr: abbr}
	if m.screen == Player {
		r.target = playerID
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, 10*time.Second)
		defer cancel()
		switch detail {
		case "plays":
			r.plays, r.err = src.Plays(ctx, game.ID, nhl.StateTTL(game), force)
		case "summary":
			r.summary, r.err = src.Summary(ctx, game.ID, nhl.StateTTL(game), force)
		case "standings":
			r.standings, r.err = src.Standings(ctx, standingsDay, force)
		case "roster":
			r.roster, r.err = src.Roster(ctx, abbr, season, force)
		case "stats":
			r.stats, r.err = src.ClubStats(ctx, abbr, season, force)
		case "player log":
			r.playerLog, r.err = src.PlayerLog(ctx, playerID, season, force)
		case "injuries":
			injuryCtx, injuryCancel := context.WithTimeout(ctx, 3*time.Second)
			defer injuryCancel()
			r.injuries, r.err = src.Injuries(injuryCtx, force)
		}
		return r
	}
}
func (m Model) scheduleTick(d time.Duration) tea.Cmd {
	generation := m.generation
	return tea.Tick(d, func(time.Time) tea.Msg { return tick{generation: generation} })
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = x.Width, x.Height
		m.feed.Width = max(20, x.Width-4)
		m.feed.Height = m.feedHeight()
		m.rebuildFeed()
		if m.gameTab == 1 {
			m.showBoxSelection()
		}
		return m, nil
	case result:
		if (x.generation != 0 && x.generation != m.generation) || x.kind != m.screen || (x.kind == Game && x.target != m.game.ID) || (x.kind == Player && x.target != 0 && x.target != m.playerID) || (x.kind == Today && x.day != "" && x.day != m.day()) || (x.kind == Team && x.teamAbbr != "" && x.teamAbbr != m.teamAbbr) || (x.kind == Leaders && (x.leadersGoalies != m.leadersGoalies || x.leadersMetric != m.leadersMetric)) {
			return m, nil
		}
		if x.detail != "" {
			m.applyDetail(x)
			return m, nil
		}
		oldTodayID, oldBoxID, oldLeaderID, oldTeamID, oldStandingID, oldTeamsID := 0, 0, m.leadersPlayerID(), 0, "", ""
		if m.selected < len(m.today) {
			oldTodayID = m.today[m.selected].ID
		}
		if m.boxSelected < len(m.boxPlayerIDs) {
			oldBoxID = m.boxPlayerIDs[m.boxSelected]
		}
		if m.standingSelected < len(m.sortedStandings()) {
			oldStandingID = m.sortedStandings()[m.standingSelected].TeamAbbrev.Default
		}
		if m.teamsSelected < len(m.sortedTeams()) {
			oldTeamsID = m.sortedTeams()[m.teamsSelected].TeamAbbrev.Default
		}
		if m.teamTab <= 1 {
			games := m.teamGames
			if m.teamTab == 1 {
				games = m.teamAllGames
			}
			if m.teamSelected < len(games) {
				oldTeamID = games[m.teamSelected].ID
			}
		} else {
			oldTeamID = m.teamSelectedPlayerID()
		}
		m.busy = false
		m.err = x.err
		var commands []tea.Cmd
		if x.err == nil {
			m.warnings = nil
			m.pending = map[string]bool{}
			m.detailErrors = map[string]string{}
			switch x.kind {
			case Today:
				m.today = x.games
				m.selected = selectionByGameID(m.today, oldTodayID, m.selected)
			case Game:
				m.game = x.game
				if x.generation == 0 {
					m.pbp = x.plays
				}
				if x.generation == 0 {
					m.summary = x.summary
				}
				m.feed.Height = m.feedHeight()
				m.rebuildFeed()
				if oldBoxID != 0 {
					for i, id := range m.boxPlayerIDs {
						if id == oldBoxID {
							m.boxSelected = i
							m.rebuildFeed()
							break
						}
					}
				}
				if m.gameTab == 1 {
					m.showBoxSelection()
				}
				if x.generation != 0 {
					for _, detail := range []string{"plays", "summary"} {
						m.pending[detail] = true
						commands = append(commands, m.loadDetail(detail, x.force))
					}
					m.rebuildFeed()
				}
			case Standings:
				m.standings = x.standings
				m.standingSelected = min(m.standingSelected, max(0, len(m.standings)-1))
				for i, row := range m.sortedStandings() {
					if row.TeamAbbrev.Default == oldStandingID && oldStandingID != "" {
						m.standingSelected = i
						break
					}
				}
			case Teams:
				m.standings = x.standings
				m.teamsSelected = min(m.teamsSelected, max(0, len(m.standings)-1))
				for i, team := range m.sortedTeams() {
					if oldTeamsID != "" && team.TeamAbbrev.Default == oldTeamsID {
						m.teamsSelected = i
						break
					}
				}
			case Team:
				if len(x.standings) != 0 {
					m.standings = x.standings
				}
				m.teamAllGames = x.games
				m.teamGames = m.teamRows(x.games)
				if x.generation == 0 {
					m.teamRoster = x.roster
				}
				if x.generation == 0 {
					m.teamStats = x.stats
				}
				if x.generation == 0 {
					m.injuries, m.injuryAvailable, m.injuryUpdated = x.injuries, true, m.now()
				}
				m.teamSelected = min(m.teamSelected, max(0, m.teamRowCount()-1))
				m.selectTeamID(oldTeamID)
				if x.generation != 0 {
					for _, detail := range []string{"standings", "roster", "stats", "injuries"} {
						m.pending[detail] = true
						commands = append(commands, m.loadDetail(detail, x.force))
					}
				}
			case Player:
				m.player = x.player
				if x.generation == 0 {
					m.playerLog = x.playerLog
				}
				if x.generation == 0 {
					m.injuries, m.injuryAvailable, m.injuryUpdated = x.injuries, true, m.now()
				}
				m.playerSelected = min(m.playerSelected, max(0, len(m.playerLog)-1))
				if x.generation != 0 {
					for _, detail := range []string{"player log", "injuries"} {
						m.pending[detail] = true
						commands = append(commands, m.loadDetail(detail, x.force))
					}
				}
			case Leaders:
				m.skaterLeaders = x.skaterLeaders
				m.goalieLeaders = x.goalieLeaders
				m.leadersSelected = min(m.leadersSelected, max(0, m.leadersCount()-1))
				if oldLeaderID != 0 {
					for i := 0; i < m.leadersCount(); i++ {
						if m.leaderID(i) == oldLeaderID {
							m.leadersSelected = i
							break
						}
					}
				}
			}
		}
		if m.screen == Game && !m.game.Final() {
			if m.game.Live() {
				commands = append(commands, m.scheduleTick(10*time.Second))
			} else {
				commands = append(commands, m.scheduleTick(30*time.Second))
			}
		}
		if m.screen == Today && m.day() == m.now().In(nhl.Eastern()).Format("2006-01-02") {
			interval := 30 * time.Second
			for _, g := range m.today {
				if g.Live() {
					interval = 10 * time.Second
					break
				}
			}
			commands = append(commands, m.scheduleTick(interval))
		}
		if len(commands) > 0 {
			return m, tea.Batch(commands...)
		}
		return m, nil
	case tick:
		if (x.generation == 0 || x.generation == m.generation) && (m.screen == Game && !m.game.Final() || m.screen == Today && m.day() == m.now().In(nhl.Eastern()).Format("2006-01-02")) {
			if !m.busy {
				m.busy = true
				return m, m.beginLoad(m.screen, false)
			}
		}
		return m, nil
	case tea.KeyMsg:
		key := x.String()
		if x.Type == tea.KeyRunes && len(x.Runes) > 1 && key != "gg" {
			var commands []tea.Cmd
			for _, letter := range x.Runes {
				next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{letter}})
				m = next.(Model)
				if cmd != nil {
					commands = append(commands, cmd)
				}
			}
			return m, tea.Batch(commands...)
		}
		// Some terminals deliver rapidly typed runes together in one key message.
		if key == "gg" && !m.help {
			m.gPending = time.Time{}
			m.gotoEnd(false)
			return m, nil
		}
		// A second g must follow promptly; any other key cancels the sequence.
		if key == "g" && !m.help {
			elapsed := m.now().Sub(m.gPending)
			if !m.gPending.IsZero() && elapsed >= 0 && elapsed <= 750*time.Millisecond {
				m.gPending = time.Time{}
				m.gotoEnd(false)
			} else {
				m.gPending = m.now()
			}
			return m, nil
		}
		m.gPending = time.Time{}
		if key == "ctrl+c" || key == "q" {
			return m, tea.Quit
		}
		if key == "?" {
			m.help = !m.help
			return m, nil
		}
		if m.help {
			if key == "esc" || key == "enter" {
				m.help = false
			}
			return m, nil
		}
		if key == "G" {
			m.gotoEnd(true)
			return m, nil
		}
		if key == "a" {
			if m.screen == Teams {
				return m, nil
			}
			m.push()
			m.teamsBack = m.screen
			if m.teamsBack == Teams {
				m.teamsBack = Today
			}
			m.screen = Teams
			m.teamsSelected = 0
			m.err = nil
			m.busy = true
			return m, m.beginLoad(Teams, false)
		}
		if key == "L" {
			if m.screen == Leaders {
				return m, nil
			}
			m.push()
			m.leadersBack = m.screen
			if m.leadersBack == Leaders {
				m.leadersBack = Today
			}
			m.screen = Leaders
			m.leadersSelected = 0
			m.skaterLeaders, m.goalieLeaders = nil, nil
			m.err = nil
			m.busy = true
			return m, m.beginLoad(Leaders, false)
		}
		if key == "esc" || key == "backspace" {
			if len(m.backStack) > 0 {
				state := m.backStack[len(m.backStack)-1]
				m.backStack = m.backStack[:len(m.backStack)-1]
				m.restore(state)
				if m.screen != Welcome {
					m.busy = true
					return m, m.beginLoad(m.screen, false)
				}
				m.busy = false
				return m, nil
			}
			switch m.screen {
			case Game:
				m.screen = m.previous
			case Standings:
				m.screen = m.standingsBack
			case Team:
				m.screen = m.teamBack
			case Player:
				m.screen = m.playerBack
			case Teams:
				m.screen = m.teamsBack
			case Leaders:
				m.screen = m.leadersBack
			}
			if m.screen == Welcome {
				m.busy = false
			}
			m.err = nil
			return m, nil
		}
		if key == "r" {
			if m.screen == Welcome {
				return m, nil
			}
			m.busy = true
			return m, m.beginLoad(m.screen, true)
		}
		switch m.screen {
		case Welcome:
			switch key {
			case "enter":
				m.push()
				m.screen = Today
				m.busy = true
				return m, m.beginLoad(Today, false)
			case "s":
				m.push()
				m.standingsBack = Welcome
				m.screen = Standings
				m.busy = true
				return m, m.beginLoad(Standings, false)
			}
		case Today:
			switch key {
			case "[", "left", "h":
				m.viewDay = m.viewDay.AddDate(0, 0, -1)
				m.selected = 0
				m.today = nil
				m.busy = true
				return m, m.beginLoad(Today, false)
			case "]", "right", "l":
				m.viewDay = m.viewDay.AddDate(0, 0, 1)
				m.selected = 0
				m.today = nil
				m.busy = true
				return m, m.beginLoad(Today, false)
			case "t":
				m.viewDay = m.now().In(nhl.Eastern())
				m.selected = 0
				m.today = nil
				m.busy = true
				return m, m.beginLoad(Today, false)
			case "up", "k":
				m.selected = max(0, m.selected-1)
			case "down", "j":
				m.selected = min(max(0, len(m.today)-1), m.selected+1)
			case "enter":
				if len(m.today) > 0 {
					m.push()
					m.game = m.today[m.selected]
					m.previous = Today
					m.screen = Game
					m.gameTab = 0
					m.keyEventsOnly = false
					m.clearGameDetail()
					m.err = nil
					m.busy = true
					return m, m.beginLoad(Game, false)
				}
			case "s":
				m.push()
				m.standingsBack = Today
				m.screen = Standings
				m.err = nil
				m.busy = true
				return m, m.beginLoad(Standings, false)
			}
		case Game:
			switch key {
			case "tab", "right", "l":
				m.gameTab = (m.gameTab + 1) % 3
				m.rebuildFeed()
				m.feed.GotoTop()
				if m.gameTab == 1 {
					m.showBoxSelection()
				}
			case "shift+tab", "left", "h":
				m.gameTab = (m.gameTab + 2) % 3
				m.rebuildFeed()
				m.feed.GotoTop()
				if m.gameTab == 1 {
					m.showBoxSelection()
				}
			case "f":
				if m.gameTab == 2 {
					m.keyEventsOnly = !m.keyEventsOnly
					m.rebuildFeed()
					m.feed.GotoTop()
				}
			case "up", "k":
				if m.gameTab == 1 {
					m.moveBox(-1)
				} else {
					m.feed.LineUp(1)
				}
			case "down", "j":
				if m.gameTab == 1 {
					m.moveBox(1)
				} else {
					m.feed.LineDown(1)
				}
			case "enter":
				if m.gameTab == 1 && m.boxSelected < len(m.boxPlayerIDs) {
					m.push()
					m.playerID = m.boxPlayerIDs[m.boxSelected]
					m.playerBack = Game
					m.playerSelected = 0
					m.clearPlayerDetail()
					m.screen = Player
					m.busy = true
					return m, m.beginLoad(Player, false)
				}
			}
		case Standings:
			rows := m.sortedStandings()
			switch key {
			case "tab", "right", "l":
				m.standingView = (m.standingView + 1) % 4
				m.standingSelected = 0
			case "shift+tab", "left", "h":
				m.standingView = (m.standingView + 3) % 4
				m.standingSelected = 0
			case "up", "k":
				m.standingSelected = max(0, m.standingSelected-1)
			case "down", "j":
				m.standingSelected = min(max(0, len(rows)-1), m.standingSelected+1)
			case "enter":
				if len(rows) > 0 {
					m.push()
					m.teamBack = Standings
					m.teamAbbr = rows[m.standingSelected].TeamAbbrev.Default
					m.teamSelected = 0
					m.teamTab = 0
					m.clearTeamDetail()
					m.screen = Team
					m.err = nil
					m.busy = true
					return m, m.beginLoad(Team, false)
				}
			}
		case Team:
			switch key {
			case "f":
				if m.teamTab == 3 {
					m.teamGoalies = !m.teamGoalies
					m.teamSelected = 0
				}
			case "tab":
				if m.teamTab == 3 {
					m.teamStatsFocus = 1 - m.teamStatsFocus
				} else {
					m.teamTab = (m.teamTab + 1) % 4
					m.teamSelected = 0
				}
			case "shift+tab":
				if m.teamTab == 3 {
					m.teamStatsFocus = 1 - m.teamStatsFocus
				} else {
					m.teamTab = (m.teamTab + 3) % 4
					m.teamSelected = 0
				}
			case "right", "l":
				if m.teamTab == 3 && m.teamStatsFocus == 1 {
					m.teamGoalies = !m.teamGoalies
					m.teamSelected = 0
					break
				}
				m.teamTab = (m.teamTab + 1) % 4
				m.teamSelected = 0
				m.teamStatsFocus = 0
			case "left", "h":
				if m.teamTab == 3 && m.teamStatsFocus == 1 {
					m.teamGoalies = !m.teamGoalies
					m.teamSelected = 0
					break
				}
				m.teamTab = (m.teamTab + 3) % 4
				m.teamSelected = 0
				m.teamStatsFocus = 0
			case "up", "k":
				m.teamSelected = max(0, m.teamSelected-1)
			case "down", "j":
				m.teamSelected = min(max(0, m.teamRowCount()-1), m.teamSelected+1)
			case "enter":
				if m.teamTab <= 1 && m.teamRowCount() > 0 {
					m.push()
					games := m.teamGames
					if m.teamTab == 1 {
						games = m.teamAllGames
					}
					m.game = games[m.teamSelected]
					m.previous = Team
					m.screen = Game
					m.gameTab = 0
					m.keyEventsOnly = false
					m.clearGameDetail()
					m.err = nil
					m.busy = true
					return m, m.beginLoad(Game, false)
				}
				id := m.teamSelectedPlayerID()
				if id > 0 {
					m.push()
					m.playerID = id
					m.playerBack = Team
					m.screen = Player
					m.playerSelected = 0
					m.clearPlayerDetail()
					m.err = nil
					m.busy = true
					return m, m.beginLoad(Player, false)
				}
			}
		case Player:
			switch key {
			case "up", "k":
				m.playerSelected = max(0, m.playerSelected-1)
			case "down", "j":
				m.playerSelected = min(max(0, len(m.playerLog)-1), m.playerSelected+1)
			case "enter":
				if len(m.playerLog) > 0 {
					m.push()
					m.game = nhl.Game{ID: m.playerLog[m.playerSelected].GameID}
					m.previous = Player
					m.screen = Game
					m.gameTab = 0
					m.clearGameDetail()
					m.err = nil
					m.busy = true
					return m, m.beginLoad(Game, false)
				}
			}
		case Teams:
			rows := m.sortedTeams()
			switch key {
			case "up", "k":
				m.teamsSelected = max(0, m.teamsSelected-1)
			case "down", "j":
				m.teamsSelected = min(max(0, len(rows)-1), m.teamsSelected+1)
			case "enter":
				if len(rows) > 0 {
					m.push()
					m.teamAbbr = rows[m.teamsSelected].TeamAbbrev.Default
					m.teamBack = Teams
					m.teamTab = 0
					m.teamSelected = 0
					m.clearTeamDetail()
					m.screen = Team
					m.busy = true
					return m, m.beginLoad(Team, false)
				}
			}
		case Leaders:
			switch key {
			case "tab", "shift+tab":
				m.leadersFocus = 1 - m.leadersFocus
			case "h", "l", "left", "right":
				if m.leadersFocus == 0 {
					m.leadersGoalies = !m.leadersGoalies
					m.leadersMetric = 0
				} else {
					count := 4
					if m.leadersGoalies {
						count = 3
					}
					step := 1
					if key == "h" || key == "left" {
						step = count - 1
					}
					m.leadersMetric = (m.leadersMetric + step) % count
				}
				m.leadersSelected = 0
				m.skaterLeaders, m.goalieLeaders = nil, nil
				m.busy = true
				return m, m.beginLoad(Leaders, false)
			case "m":
				count := 4
				if m.leadersGoalies {
					count = 3
				}
				m.leadersMetric = (m.leadersMetric + 1) % count
				m.leadersSelected = 0
				m.skaterLeaders, m.goalieLeaders = nil, nil
				m.busy = true
				return m, m.beginLoad(Leaders, false)
			case "u":
				if m.leadersGoalies {
					m.goalieQualified = !m.goalieQualified
					m.leadersSelected = 0
				}
			case "up", "k":
				m.leadersSelected = max(0, m.leadersSelected-1)
			case "down", "j":
				m.leadersSelected = min(max(0, m.leadersCount()-1), m.leadersSelected+1)
			case "enter":
				id := m.leadersPlayerID()
				if id > 0 {
					m.push()
					m.playerID = id
					m.playerBack = Leaders
					m.playerSelected = 0
					m.clearPlayerDetail()
					m.screen = Player
					m.busy = true
					return m, m.beginLoad(Player, false)
				}
			}
		}
	}
	return m, nil
}
func (m *Model) applyDetail(x result) {
	delete(m.pending, x.detail)
	if m.detailErrors == nil {
		m.detailErrors = map[string]string{}
	}
	if x.err != nil {
		labels := map[string]string{"plays": "Play-by-play", "summary": "Summary", "standings": "Team record", "roster": "Roster", "stats": "Stats", "player log": "Game log", "injuries": "Injuries"}
		m.detailErrors[x.detail] = labels[x.detail] + " unavailable"
	} else {
		delete(m.detailErrors, x.detail)
		switch x.detail {
		case "plays":
			m.pbp = x.plays
			m.feed.Height = m.feedHeight()
			m.rebuildFeed()
		case "summary":
			m.summary = x.summary
			m.rebuildFeed()
		case "standings":
			m.standings = x.standings
		case "roster", "stats":
			oldID := m.teamSelectedPlayerID()
			if x.detail == "roster" {
				m.teamRoster = x.roster
			} else {
				m.teamStats = x.stats
			}
			m.teamSelected = min(m.teamSelected, max(0, m.teamRowCount()-1))
			m.selectTeamID(oldID)
		case "player log":
			oldID := 0
			if m.playerSelected < len(m.playerLog) {
				oldID = m.playerLog[m.playerSelected].GameID
			}
			m.playerLog = x.playerLog
			for i, game := range m.playerLog {
				if game.GameID == oldID && oldID != 0 {
					m.playerSelected = i
					break
				}
			}
			m.playerSelected = min(m.playerSelected, max(0, len(m.playerLog)-1))
		case "injuries":
			m.injuries, m.injuryAvailable, m.injuryUpdated = x.injuries, true, m.now()
		}
	}
	m.warnings = nil
	for _, detail := range []string{"plays", "summary", "standings", "roster", "stats", "player log", "injuries"} {
		if label := m.detailErrors[detail]; label != "" {
			m.warnings = append(m.warnings, label)
		}
	}
}
func (m *Model) push() {
	m.backStack = append(m.backStack, navigationState{
		screen: m.screen, viewDay: m.viewDay, today: m.today, selected: m.selected,
		game: m.game, pbp: m.pbp, summary: m.summary, gameTab: m.gameTab,
		boxSelected: m.boxSelected, keyEventsOnly: m.keyEventsOnly, feedOffset: m.feed.YOffset,
		standings: m.standings, standingView: m.standingView, standingSelected: m.standingSelected,
		teamAbbr: m.teamAbbr, teamGames: m.teamGames, teamAllGames: m.teamAllGames,
		teamRoster: m.teamRoster, teamStats: m.teamStats, injuries: m.injuries,
		injuryUpdated: m.injuryUpdated, injuryAvailable: m.injuryAvailable,
		teamTab: m.teamTab, teamStatsFocus: m.teamStatsFocus, teamGoalies: m.teamGoalies,
		teamSelected: m.teamSelected, playerID: m.playerID, player: m.player,
		playerLog: m.playerLog, playerSelected: m.playerSelected, teamsSelected: m.teamsSelected,
		leadersGoalies: m.leadersGoalies, goalieQualified: m.goalieQualified,
		leadersMetric: m.leadersMetric, leadersFocus: m.leadersFocus, leadersSelected: m.leadersSelected,
		skaterLeaders: m.skaterLeaders, goalieLeaders: m.goalieLeaders,
		err: m.err, warnings: append([]string(nil), m.warnings...),
	})
}
func (m *Model) restore(s navigationState) {
	m.screen, m.viewDay, m.today, m.selected = s.screen, s.viewDay, s.today, s.selected
	m.game, m.pbp, m.summary, m.gameTab = s.game, s.pbp, s.summary, s.gameTab
	m.boxSelected, m.keyEventsOnly = s.boxSelected, s.keyEventsOnly
	m.standings, m.standingView, m.standingSelected = s.standings, s.standingView, s.standingSelected
	m.teamAbbr, m.teamGames, m.teamAllGames = s.teamAbbr, s.teamGames, s.teamAllGames
	m.teamRoster, m.teamStats, m.injuries = s.teamRoster, s.teamStats, s.injuries
	m.injuryUpdated, m.injuryAvailable = s.injuryUpdated, s.injuryAvailable
	m.teamTab, m.teamStatsFocus, m.teamGoalies, m.teamSelected = s.teamTab, s.teamStatsFocus, s.teamGoalies, s.teamSelected
	m.playerID, m.player, m.playerLog, m.playerSelected = s.playerID, s.player, s.playerLog, s.playerSelected
	m.teamsSelected, m.leadersGoalies, m.goalieQualified = s.teamsSelected, s.leadersGoalies, s.goalieQualified
	m.leadersMetric, m.leadersFocus, m.leadersSelected = s.leadersMetric, s.leadersFocus, s.leadersSelected
	m.skaterLeaders, m.goalieLeaders = s.skaterLeaders, s.goalieLeaders
	m.err, m.warnings = s.err, s.warnings
	m.detailErrors = nil
	m.feed.Width = max(20, m.width-4)
	m.feed.Height = m.feedHeight()
	m.rebuildFeed()
	m.feed.SetYOffset(s.feedOffset)
	if m.screen == Game && m.gameTab == 1 {
		m.showBoxSelection()
	}
}
func (m *Model) clearGameDetail() {
	m.warnings = nil
	m.pending, m.detailErrors = nil, nil
	m.pbp = nhl.PBP{}
	m.summary = nhl.GameSummary{}
	m.feed.SetContent("Loading game…")
	m.feed.GotoTop()
	m.boxSelected = 0
	m.boxPlayerIDs = nil
	m.boxLineOffsets = nil
}
func (m *Model) moveBox(delta int) {
	if len(m.boxPlayerIDs) == 0 {
		return
	}
	m.boxSelected = min(max(0, m.boxSelected+delta), len(m.boxPlayerIDs)-1)
	m.rebuildFeed()
	m.showBoxSelection()
}
func (m *Model) showBoxSelection() {
	if m.boxSelected >= len(m.boxLineOffsets) {
		return
	}
	line := m.boxLineOffsets[m.boxSelected]
	if line < m.feed.YOffset {
		m.feed.SetYOffset(line)
	}
	if line >= m.feed.YOffset+m.feed.Height {
		m.feed.SetYOffset(line - m.feed.Height + 1)
	}
}
func (m *Model) clearTeamDetail() {
	m.warnings = nil
	m.pending, m.detailErrors = nil, nil
	m.injuries = nil
	m.injuryAvailable = false
	m.injuryUpdated = time.Time{}
	m.teamGames = nil
	m.teamAllGames = nil
	m.teamRoster = nhl.Roster{}
	m.teamStats = nhl.ClubStats{}
	m.teamGoalies = false
}
func (m *Model) clearPlayerDetail() {
	m.warnings = nil
	m.pending, m.detailErrors = nil, nil
	m.injuries = nil
	m.injuryAvailable = false
	m.injuryUpdated = time.Time{}
	m.player = nhl.PlayerProfile{}
	m.playerLog = nil
}
func (m *Model) gotoEnd(bottom bool) {
	switch m.screen {
	case Today:
		m.selected = 0
		if bottom {
			m.selected = max(0, len(m.today)-1)
		}
	case Game:
		if m.gameTab == 1 && len(m.boxPlayerIDs) > 0 {
			m.boxSelected = 0
			if bottom {
				m.boxSelected = len(m.boxPlayerIDs) - 1
			}
			m.rebuildFeed()
			m.showBoxSelection()
		} else if bottom {
			m.feed.GotoBottom()
		} else {
			m.feed.GotoTop()
		}
	case Standings:
		m.standingSelected = 0
		if bottom {
			m.standingSelected = max(0, len(m.standings)-1)
		}
	case Team:
		m.teamSelected = 0
		if bottom {
			m.teamSelected = max(0, m.teamRowCount()-1)
		}
	case Player:
		m.playerSelected = 0
		if bottom {
			m.playerSelected = max(0, len(m.playerLog)-1)
		}
	case Teams:
		m.teamsSelected = 0
		if bottom {
			m.teamsSelected = max(0, len(m.standings)-1)
		}
	case Leaders:
		m.leadersSelected = 0
		if bottom {
			m.leadersSelected = max(0, m.leadersCount()-1)
		}
	}
}
func (m Model) leadersCount() int {
	if m.leadersGoalies {
		return len(m.visibleGoalieLeaders())
	}
	return len(m.skaterLeaders)
}
func (m Model) goalieMinimumGames() int {
	most := 0
	for _, goalie := range m.goalieLeaders {
		most = max(most, goalie.GP)
	}
	return max(2, most/2)
}
func (m Model) visibleGoalieLeaders() []nhl.LeaderGoalie {
	if !m.goalieQualified {
		return m.goalieLeaders
	}
	minimum := m.goalieMinimumGames()
	rows := make([]nhl.LeaderGoalie, 0, len(m.goalieLeaders))
	for _, goalie := range m.goalieLeaders {
		if goalie.GP >= minimum {
			rows = append(rows, goalie)
		}
	}
	return rows
}
func (m Model) leadersPlayerID() int {
	return m.leaderID(m.leadersSelected)
}
func (m Model) leaderID(index int) int {
	if m.leadersGoalies {
		rows := m.visibleGoalieLeaders()
		if index >= 0 && index < len(rows) {
			return rows[index].ID
		}
	} else if index >= 0 && index < len(m.skaterLeaders) {
		return m.skaterLeaders[index].ID
	}
	return 0
}
func selectionByGameID(rows []nhl.Game, id, fallback int) int {
	if id != 0 {
		for i, row := range rows {
			if row.ID == id {
				return i
			}
		}
	}
	return min(fallback, max(0, len(rows)-1))
}
func (m *Model) selectTeamID(id int) {
	if id == 0 {
		return
	}
	if m.teamTab <= 1 {
		rows := m.teamGames
		if m.teamTab == 1 {
			rows = m.teamAllGames
		}
		m.teamSelected = selectionByGameID(rows, id, m.teamSelected)
		return
	}
	for i := 0; i < m.teamRowCount(); i++ {
		m.teamSelected = i
		if m.teamSelectedPlayerID() == id {
			return
		}
	}
	m.teamSelected = 0
}
func (m Model) teamRowCount() int {
	switch m.teamTab {
	case 0:
		return len(m.teamGames)
	case 1:
		return len(m.teamAllGames)
	case 2:
		return len(m.teamRoster.Players())
	default:
		if m.teamGoalies {
			return len(m.teamStats.Goalies)
		}
		return len(m.teamStats.Skaters)
	}
}
func (m Model) teamSelectedPlayerID() int {
	if m.teamTab == 2 {
		players := m.sortedRosterPlayers()
		if m.teamSelected < len(players) {
			return players[m.teamSelected].ID
		}
	}
	if m.teamTab == 3 {
		if m.teamGoalies && m.teamSelected < len(m.teamStats.Goalies) {
			return m.sortedTeamGoalies()[m.teamSelected].ID
		}
		if !m.teamGoalies && m.teamSelected < len(m.teamStats.Skaters) {
			return m.sortedTeamSkaters()[m.teamSelected].ID
		}
	}
	return 0
}

func (m Model) sortedTeamSkaters() []nhl.ClubSkater {
	rows := append([]nhl.ClubSkater(nil), m.teamStats.Skaters...)
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Points != b.Points {
			return a.Points > b.Points
		}
		if a.Goals != b.Goals {
			return a.Goals > b.Goals
		}
		return a.FullName() < b.FullName()
	})
	return rows
}

func (m Model) sortedTeamGoalies() []nhl.ClubGoalie {
	rows := append([]nhl.ClubGoalie(nil), m.teamStats.Goalies...)
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.SavePct != b.SavePct {
			return a.SavePct > b.SavePct
		}
		if a.GP != b.GP {
			return a.GP > b.GP
		}
		return a.FullName() < b.FullName()
	})
	return rows
}

func (m Model) sortedRosterPlayers() []nhl.RosterPlayer {
	points := make(map[int]int, len(m.teamStats.Skaters))
	for _, p := range m.teamStats.Skaters {
		points[p.ID] = p.Points
	}
	savePct := make(map[int]float64, len(m.teamStats.Goalies))
	for _, p := range m.teamStats.Goalies {
		savePct[p.ID] = p.SavePct
	}
	skaters := append(append([]nhl.RosterPlayer(nil), m.teamRoster.Forwards...), m.teamRoster.Defense...)
	goalies := append([]nhl.RosterPlayer(nil), m.teamRoster.Goalies...)
	sort.SliceStable(skaters, func(i, j int) bool {
		if points[skaters[i].ID] != points[skaters[j].ID] {
			return points[skaters[i].ID] > points[skaters[j].ID]
		}
		return skaters[i].FullName() < skaters[j].FullName()
	})
	sort.SliceStable(goalies, func(i, j int) bool {
		if savePct[goalies[i].ID] != savePct[goalies[j].ID] {
			return savePct[goalies[i].ID] > savePct[goalies[j].ID]
		}
		return goalies[i].FullName() < goalies[j].FullName()
	})
	return append(skaters, goalies...)
}

func injuryName(name string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (m Model) injuryStatus(team, name string) string {
	if team == "" || name == "" {
		return ""
	}
	key := injuryName(name)
	for _, injury := range m.injuries {
		if injury.Team == team && injuryName(injury.Player) == key {
			return injury.Status
		}
	}
	return ""
}
func (m Model) injurySource() string {
	if m.injuryAvailable {
		label := "Injuries: ESPN checked " + m.injuryUpdated.In(nhl.Eastern()).Format("Jan 02 3:04 PM") + " ET"
		for _, warning := range m.warnings {
			if warning == "Injuries unavailable" {
				return label + " · last successful check"
			}
		}
		return label
	}
	if m.busy || m.pending["injuries"] {
		return "Injuries: ESPN loading"
	}
	return "Injuries: ESPN unavailable"
}
func (m Model) teamRows(all []nhl.Game) []nhl.Game {
	now := m.now()
	past := []nhl.Game{}
	future := []nhl.Game{}
	for _, g := range all {
		if g.GameType != 0 && g.GameType != 2 {
			continue
		}
		if g.Final() || g.StartUTC.Before(now) && !g.Live() {
			past = append(past, g)
		} else {
			future = append(future, g)
		}
	}
	if len(past) > 5 {
		past = past[len(past)-5:]
	}
	if len(future) > 7 {
		future = future[:7]
	}
	rows := append(past, future...)
	nhl.SortGames(rows)
	return rows
}
func (m Model) sortedStandings() []nhl.Standing {
	rows := append([]nhl.Standing(nil), m.standings...)
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if m.standingView == standingsLeague {
			return a.LeagueSequence < b.LeagueSequence
		}
		if a.Conference != b.Conference {
			return a.Conference < b.Conference
		}
		if m.standingView == standingsDivision {
			if a.Division != b.Division {
				return a.Division < b.Division
			}
			return a.DivisionSequence < b.DivisionSequence
		}
		if m.standingView == standingsWildCard {
			if (a.WildcardSequence == 0) != (b.WildcardSequence == 0) {
				return a.WildcardSequence == 0
			}
			if a.WildcardSequence == 0 {
				if a.Division != b.Division {
					return a.Division < b.Division
				}
				return a.DivisionSequence < b.DivisionSequence
			}
			return a.WildcardSequence < b.WildcardSequence
		}
		return a.ConferenceSequence < b.ConferenceSequence
	})
	return rows
}
func standingsWindow(rows []nhl.Standing, selected, available int, separateConferences bool) (start, end int, showSections bool) {
	available = max(1, available)
	if separateConferences && available > 1 {
		for count := available; count > 0; count-- {
			start = windowStart(selected, len(rows), count)
			end = min(len(rows), start+count)
			sections := 0
			lastConference := ""
			for _, row := range rows[start:end] {
				if row.Conference != lastConference {
					sections++
					lastConference = row.Conference
				}
			}
			if count+2*sections <= available {
				return start, end, true
			}
		}
	}
	start = windowStart(selected, len(rows), available)
	return start, min(len(rows), start+available), false
}
func (m Model) sortedTeams() []nhl.Standing {
	rows := append([]nhl.Standing(nil), m.standings...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].TeamName.Default < rows[j].TeamName.Default })
	return rows
}
func (m *Model) rebuildFeed() {
	lines := []string{}
	switch m.gameTab {
	case 0:
		if len(m.summary.Broadcasts) > 0 {
			networks := []string{}
			seen := map[string]bool{}
			for _, b := range m.summary.Broadcasts {
				if b.Network != "" && !seen[b.Network] {
					networks = append(networks, b.Network)
					seen[b.Network] = true
				}
			}
			if len(networks) > 0 {
				lines = append(lines, "BROADCAST  "+strings.Join(networks, " · "), "")
			}
		}
		for _, period := range m.summary.Summary.Scoring {
			for _, goal := range period.Goals {
				assists := []string{}
				for _, a := range goal.Assists {
					assists = append(assists, a.Name.Default)
				}
				line := fmt.Sprintf("%-3s %-5s %-3s GOAL  %s", periodLabel(period.Period), goal.Time, goal.TeamAbbrev.Default, goal.Name.Default)
				if len(assists) > 0 {
					line += "  · " + strings.Join(assists, ", ")
				}
				lines = append(lines, line)
			}
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		for _, period := range m.summary.Summary.Penalties {
			for _, p := range period.Penalties {
				name := strings.TrimSpace(p.CommittedBy.First.Default + " " + p.CommittedBy.Last.Default)
				lines = append(lines, fmt.Sprintf("%-3s %-5s %-3s PEN   %s · %s %d min", periodLabel(period.Period), p.Time, p.TeamAbbrev.Default, name, strings.ReplaceAll(p.Description, "-", " "), p.Duration))
			}
		}
		if len(m.summary.Summary.ThreeStars) > 0 {
			lines = append(lines, "", "THREE STARS")
			for _, star := range m.summary.Summary.ThreeStars {
				lines = append(lines, fmt.Sprintf("%d  %-3s  %s", star.Star, star.TeamAbbrev, star.Name.Default))
			}
		}
	case 1:
		m.boxPlayerIDs, m.boxLineOffsets = nil, nil
		count := len(m.game.PlayerStats.Away.Forwards) + len(m.game.PlayerStats.Away.Defense) + len(m.game.PlayerStats.Away.Goalies) + len(m.game.PlayerStats.Home.Forwards) + len(m.game.PlayerStats.Home.Defense) + len(m.game.PlayerStats.Home.Goalies)
		if m.boxSelected >= count {
			m.boxSelected = max(0, count-1)
		}
		addPlayer := func(line string, id int) {
			if id > 0 {
				line = m.row(line, len(m.boxPlayerIDs) == m.boxSelected, false)
				m.boxPlayerIDs = append(m.boxPlayerIDs, id)
				m.boxLineOffsets = append(m.boxLineOffsets, len(lines))
			}
			lines = append(lines, line)
		}
		for _, side := range []struct {
			abbr  string
			group nhl.GamePlayerGroup
		}{{m.game.Away.Abbrev, m.game.PlayerStats.Away}, {m.game.Home.Abbrev, m.game.PlayerStats.Home}} {
			lines = append(lines, side.abbr+" SKATERS", m.rowHeader(columnsRight([]int{0, 28, 32, 36, 40, 45, 50, 56, 62}, 1, 7, "PLAYER", "G", "A", "P", "+/-", "PIM", "SOG", "HIT", "TOI")))
			skaters := append(append([]nhl.Skater{}, side.group.Forwards...), side.group.Defense...)
			sort.SliceStable(skaters, func(i, j int) bool {
				a, b := skaters[i], skaters[j]
				if a.Points != b.Points {
					return a.Points > b.Points
				}
				if a.Goals != b.Goals {
					return a.Goals > b.Goals
				}
				return a.Name.Default < b.Name.Default
			})
			for _, p := range skaters {
				addPlayer(columnsRight([]int{0, 28, 32, 36, 40, 45, 50, 56, 62}, 1, 7, fmt.Sprintf("#%d %s", p.Number, p.Name.Default), fmt.Sprint(p.Goals), fmt.Sprint(p.Assists), fmt.Sprint(p.Points), fmt.Sprintf("%+d", p.PlusMinus), fmt.Sprint(p.PIM), fmt.Sprint(p.Shots), fmt.Sprint(p.Hits), p.TOI), p.ID)
			}
			starts := []int{0, 28, 35, 41, 48, 56}
			lines = append(lines, "", side.abbr+" GOALIES", m.rowHeader(columnsRight(starts, 1, 4, "PLAYER", "SV", "SA", "GA", "SV%", "TOI")))
			goalies := append([]nhl.Goalie(nil), side.group.Goalies...)
			sort.SliceStable(goalies, func(i, j int) bool {
				a, b := goalieSavePct(goalies[i]), goalieSavePct(goalies[j])
				if a != b {
					return a > b
				}
				return goalies[i].Saves > goalies[j].Saves
			})
			for _, p := range goalies {
				pct := ""
				if p.ShotsAgainst > 0 {
					pct = fmt.Sprintf("%.3f", goalieSavePct(p))
				}
				addPlayer(columnsRight(starts, 1, 4, fmt.Sprintf("#%d %s", p.Number, p.Name.Default), fmt.Sprint(p.Saves), fmt.Sprint(p.ShotsAgainst), fmt.Sprint(p.GoalsAgainst), pct, p.TOI), p.ID)
			}
			lines = append(lines, "")
		}
	case 2:
		for _, p := range m.pbp.Plays {
			if m.keyEventsOnly && p.Type != "goal" && p.Type != "penalty" && p.Type != "period-start" && p.Type != "period-end" {
				continue
			}
			if line := m.playLine(p); line != "" {
				lines = append(lines, line)
			}
		}
	}
	if len(lines) == 0 {
		message := "No data available yet."
		if m.gameTab == 0 && m.pending["summary"] {
			message = "Loading summary…"
		}
		if m.gameTab == 2 && m.pending["plays"] {
			message = "Loading play-by-play…"
		}
		lines = []string{message}
	}
	m.feed.SetContent(strings.Join(lines, "\n"))
}
func goalieSavePct(p nhl.Goalie) float64 {
	if p.ShotsAgainst <= 0 {
		return 0
	}
	return float64(p.Saves) / float64(p.ShotsAgainst)
}
func (m Model) feedHeight() int {
	header := 6 // score, status, tabs, spacer, section, breathing room
	if m.game.Away.Shots != nil && m.game.Home.Shots != nil {
		header++
	}
	if m.game.Live() && len(m.pbp.Plays) > 0 {
		code := m.pbp.Plays[len(m.pbp.Plays)-1].Situation
		if manpowerLabel(code, m.game.Away.Abbrev, m.game.Home.Abbrev) != "" {
			header++
		}
	}
	if m.game.Venue.Default != "" && m.height > 20 {
		header++
	}
	return max(1, m.contentHeight()-header)
}
func (m Model) playLine(p nhl.Play) string {
	label := fmt.Sprintf("%s %s", periodLabel(p.Period), p.Time)
	d := p.Details
	team := ""
	if d.TeamID != 0 && d.TeamID == m.game.Away.ID {
		team = m.game.Away.Abbrev
	}
	if d.TeamID != 0 && d.TeamID == m.game.Home.ID {
		team = m.game.Home.Abbrev
	}
	switch p.Type {
	case "goal":
		assists := []string{}
		for _, id := range []int{d.Assist1, d.Assist2} {
			if id > 0 {
				assists = append(assists, m.pbp.Player(id))
			}
		}
		extra := ""
		if len(assists) > 0 {
			extra = " (" + strings.Join(assists, ", ") + ")"
		}
		score := ""
		if d.AwayScore != nil && d.HomeScore != nil {
			score = fmt.Sprintf(" %d–%d", *d.AwayScore, *d.HomeScore)
		}
		strength := ""
		if label := manpowerLabel(p.Situation, m.game.Away.Abbrev, m.game.Home.Abbrev); label != "" {
			strength = " · " + label
		}
		return fmt.Sprintf("%-12s GOAL %-3s %s%s%s%s", label, team, m.pbp.Player(d.Scorer), extra, score, strength)
	case "penalty":
		return fmt.Sprintf("%-12s PEN  %-3s %s · %s %d min", label, team, m.pbp.Player(d.CommittedBy), strings.ReplaceAll(d.Infraction, "-", " "), d.Minutes)
	case "period-start":
		return fmt.Sprintf("%-12s Period begins", label)
	case "period-end":
		return fmt.Sprintf("%-12s Period ends", label)
	}
	names := map[string]string{"shot-on-goal": "SHOT", "missed-shot": "MISS", "blocked-shot": "BLOCK", "faceoff": "FACEOFF", "hit": "HIT", "giveaway": "GIVEAWAY", "takeaway": "TAKEAWAY", "stoppage": "STOP", "game-end": "GAME END", "delayed-penalty": "DELAYED PEN"}
	name := names[p.Type]
	if name == "" {
		name = strings.ToUpper(strings.ReplaceAll(p.Type, "-", " "))
	}
	player := ""
	for _, id := range []int{d.Shooter, d.Winner, d.Hitter, d.BlockedBy, d.PlayerID} {
		if id > 0 {
			player = m.pbp.Player(id)
			break
		}
	}
	return fmt.Sprintf("%-12s %-11s %-3s %s", label, name, team, player)
}
func periodLabel(p nhl.Period) string {
	if p.Type == "OT" || p.Type == "SO" {
		return p.Type
	}
	return fmt.Sprintf("P%d", p.Number)
}
func manpowerLabel(code, away, home string) string {
	if len(code) != 4 || code[1] == code[2] {
		return ""
	}
	advantaged := away
	if code[2] > code[1] {
		advantaged = home
	}
	if code[0] == '0' || code[3] == '0' {
		return advantaged + " extra attacker"
	}
	if code[0] == '1' && code[3] == '1' {
		return advantaged + " power play"
	}
	return ""
}

var title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#abb2bf"))
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#7f8795"))
var live = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e5c07b"))
var selected = lipgloss.NewStyle().Background(lipgloss.Color("#3e4451")).Foreground(lipgloss.Color("#ffffff"))

// Columns keeps the start of each field stable, including when a value is empty.
func columns(starts []int, values ...string) string {
	var b strings.Builder
	position := 0
	for i, value := range values {
		if i >= len(starts) {
			break
		}
		if position < starts[i] {
			b.WriteString(strings.Repeat(" ", starts[i]-position))
			position = starts[i]
		}
		if i+1 < len(starts) {
			value = clipColumn(value, max(0, starts[i+1]-starts[i]-1))
		}
		b.WriteString(value)
		position += lipgloss.Width(value)
	}
	return b.String()
}
func columnsRight(starts []int, first, last int, values ...string) string {
	items := append([]string(nil), values...)
	for i := first; i <= last && i < len(items) && i < len(starts); i++ {
		width := 5
		if i+1 < len(starts) {
			width = max(0, starts[i+1]-starts[i]-1)
		}
		items[i] = fmt.Sprintf("%*s", width, clipColumn(items[i], width))
	}
	return columns(starts, items...)
}
func optionTabs(options []string, selected int) string {
	tabs := append([]string(nil), options...)
	for i := range tabs {
		if i == selected {
			tabs[i] = "[" + tabs[i] + "]"
		}
	}
	return strings.Join(tabs, "   ")
}
func clipColumn(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if lipgloss.Width(b.String()+string(r)+"…") > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
}

// Keep schedules compact on wide terminals while giving the final field room at 80 columns.
func scheduleResultStart(width int) int { return min(42, max(32, width/2)) }
func (m Model) footer() []string {
	parts := []string{"j/k move", "gg/G top/bottom"}
	if m.screen == Welcome {
		parts = []string{"enter Today", "a teams", "L leaders", "s standings", "? help", "q quit"}
	} else {
		switch m.screen {
		case Today, Standings, Team, Player, Teams, Leaders:
			parts = append(parts, "enter open")
		}
		if m.screen != Today {
			parts = append(parts, "esc back")
		}
		if m.screen == Standings {
			parts = append(parts, "tab/h/l view")
		}
		if m.screen == Today {
			parts = append(parts, "h/l date", "t today", "s standings")
		}
		if m.screen == Game {
			parts = append(parts, "tab/h/l view")
			if m.gameTab == 2 {
				parts = append(parts, "f key events")
			}
			if m.gameTab == 1 {
				parts = append(parts, "enter player")
			}
		}
		if m.screen == Team {
			parts = append(parts, "h/l panel")
			if m.teamTab == 3 {
				parts = append(parts, "tab focus", "h/l option")
			}
		}
		if m.screen == Leaders {
			parts = append(parts, "tab focus", "h/l option")
			if m.leadersGoalies {
				parts = append(parts, "u qualify")
			}
		}
		if m.screen != Teams {
			parts = append(parts, "a teams")
		}
		if m.screen != Leaders {
			parts = append(parts, "L leaders")
		}
		parts = append(parts, "r refresh", "? help", "q quit")
	}
	width := max(1, m.width)
	var lines []string
	line := ""
	for _, part := range parts {
		if len(line)+len(part)+3 > width && line != "" {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " · "
		}
		line += part
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
func (m Model) contentHeight() int {
	status := 0
	if m.err != nil || len(m.warnings) > 0 {
		status = 1
	}
	return max(0, m.height-len(m.footer())-1-status)
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	var lines []string
	switch m.screen {
	case Welcome:
		lines = m.welcomeView()
	case Today:
		lines = m.todayView()
	case Game:
		lines = m.gameView()
	case Standings:
		lines = m.standingsView()
	case Team:
		lines = m.teamView()
	case Player:
		lines = m.playerView()
	case Teams:
		lines = m.teamsView()
	case Leaders:
		lines = m.leadersView()
	}
	status := ""
	if m.err != nil {
		status = "Last refresh failed · press r to retry"
		if len(lines) < 3 {
			status = "NHL API unavailable · press r to retry"
		}
	} else if len(m.warnings) > 0 {
		status = strings.Join(m.warnings, " · ")
	}
	if status != "" {
		if len(lines) == 0 {
			lines = []string{status}
		} else {
			lines = append(lines[:1], append([]string{live.Render(status)}, lines[1:]...)...)
		}
	}
	if m.busy && len(lines) > 0 && m.width >= 50 {
		lines[0] += muted.Render("  ·  Updating…")
	}
	if m.busy && len(lines) < 3 {
		lines = append(lines, muted.Render("Loading…"))
	}
	if m.help {
		lines = []string{title.Render("NHL Stats Vim · Keys"), "", "Vim-style navigation on every list and feed:", "j/k or ↓/↑  move down/up", "gg / G       first/last item", "", "enter  open selected game, team, or player", "esc/backspace  back", "Today: [/] or h/l dates · t today", "Game: tab or h/l views · f key events", "Game box score: enter opens player", "Team Stats: tab focuses option row · h/l changes it", "Leaders: tab focuses group or metric · h/l changes it", "Goalie leaders: u toggles qualified list", "a  all teams · L  leaders", "r  refresh · ?  close help · q  quit"}
		return m.joinClipped(m.centeredLines(lines, m.height))
	}
	lines = strings.Split(strings.Join(lines, "\n"), "\n")
	limit := max(0, m.height-len(m.footer())-1)
	if len(lines) > limit {
		lines = lines[:limit]
	}
	for len(lines) < limit {
		lines = append(lines, "")
	}
	if m.height > 0 {
		lines = append(lines, "")
	}
	for _, line := range m.footer() {
		lines = append(lines, muted.Render(line))
	}
	if len(lines) > m.height {
		lines = lines[len(lines)-m.height:]
	}
	return m.joinClipped(lines)
}
func (m Model) joinClipped(lines []string) string {
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], max(1, m.width), "")
	}
	return strings.Join(lines, "\n")
}
func (m Model) centeredLines(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	centered := make([]string, max(0, (height-len(lines))/2))
	for _, line := range lines {
		centered = append(centered, lipgloss.PlaceHorizontal(max(1, m.width), lipgloss.Center, line))
	}
	for len(centered) < height {
		centered = append(centered, "")
	}
	return centered
}
func (m Model) welcomeView() []string {
	subtitle, features, navigation, prompt := "NHL Hockey in your terminal", "Scores · Gamecenter · teams · leaders", "Vim-style navigation throughout", "Press enter for today's games"
	if m.width < 40 {
		features, navigation, prompt = "Scores · games", "Vim-style keys", "Enter for Today"
	}
	if m.width < 24 {
		subtitle = "Hockey TUI"
	}
	lines := []string{
		title.Render("NHL Stats Vim"),
		muted.Render(subtitle),
		"",
		features,
		muted.Render(navigation),
		"",
		prompt,
	}
	return m.centeredLines(lines, m.contentHeight())
}
func (m Model) todayView() []string {
	lines := []string{title.Render("NHL Stats Vim  ·  Scores  ·  " + m.viewDay.Format("Mon, Jan 2, 2006") + " ET"), ""}
	if len(m.today) == 0 && m.err == nil && !m.busy {
		return append(lines, muted.Render("No games today"))
	}
	visible := max(1, m.contentHeight()-2)
	start := windowStart(m.selected, len(m.today), visible)
	end := min(len(m.today), start+visible)
	for i := start; i < end; i++ {
		g := m.today[i]
		statusStart := scheduleResultStart(m.width)
		text := columns([]int{0, 7, 14, 21, statusStart}, g.Away.Abbrev, g.Score(g.Away), g.Home.Abbrev, g.Score(g.Home), g.Status())
		lines = append(lines, m.row(text, i == m.selected, g.Live()))
	}
	return lines
}
func windowStart(sel, total, visible int) int {
	if total <= visible {
		return 0
	}
	return min(max(0, sel-visible+1), total-visible)
}
func (m Model) row(s string, active, hot bool) string {
	gutter := min(3, max(0, m.width))
	prefix := strings.Repeat(" ", gutter)
	if active && gutter > 0 {
		prefix = selected.Render("›" + strings.Repeat(" ", gutter-1))
	}
	s = ansi.Truncate(s, max(1, m.width-gutter), "")
	if active {
		return prefix + selected.Render(s)
	}
	if hot {
		return prefix + live.Render(s)
	}
	return prefix + s
}
func (m Model) rowHeader(s string) string {
	return ansi.Truncate("   "+s, max(1, m.width), "")
}
func (m Model) gameView() []string {
	g := m.game
	lines := []string{title.Render(fmt.Sprintf("%s %s  —  %s %s", g.Away.Abbrev, g.Score(g.Away), g.Home.Abbrev, g.Score(g.Home))), live.Render(g.Status())}
	if g.Away.Shots != nil && g.Home.Shots != nil {
		lines = append(lines, muted.Render(fmt.Sprintf("Shots %d–%d", *g.Away.Shots, *g.Home.Shots)))
	}
	if g.Live() && len(m.pbp.Plays) > 0 {
		if label := manpowerLabel(m.pbp.Plays[len(m.pbp.Plays)-1].Situation, g.Away.Abbrev, g.Home.Abbrev); label != "" {
			lines = append(lines, live.Render(label))
		}
	}
	if g.Venue.Default != "" && m.height > 20 {
		lines = append(lines, muted.Render(g.Venue.Default))
	}
	lines = append(lines, muted.Render(optionTabs([]string{"Summary", "Box Score", "Play-by-Play"}, m.gameTab)))
	section := []string{"Game summary", "Player box score", "All events"}[m.gameTab]
	if m.gameTab == 2 && m.keyEventsOnly {
		section = "Key events"
	}
	lines = append(lines, "", title.Render(section), "")
	if m.err != nil && len(m.pbp.Plays) == 0 {
		lines = append(lines, "Game data unavailable")
	} else {
		m.feed.Height = m.feedHeight()
		lines = append(lines, m.feed.View())
	}
	return lines
}
func (m Model) standingsView() []string {
	options := []string{"Wild Card", "Division", "Conference", "League"}
	mode := options[m.standingView]
	starts := []int{0, 14, 22, 28, 34, 40, 48}
	head := []string{"GROUP", "TEAM", " GP", "  W", "  L", "OTL", " PTS"}
	if m.standingView == standingsWildCard {
		head[0] = "PLACE"
	}
	if m.width >= 80 {
		starts = append(starts, 55, 62, 68, 74)
		head = append(head, "  P%", " RW", " GF", " GA")
	}
	if m.width >= 100 {
		starts = append(starts, 81, 90)
		head = append(head, "DIFF", "STREAK")
	}
	lastNumber := len(head) - 1
	if m.width >= 100 {
		lastNumber--
	}
	lines := []string{title.Render("Standings · " + mode), muted.Render(optionTabs(options, m.standingView)), "", m.rowHeader(columnsRight(starts, 2, lastNumber, head...)), ""}
	rows := m.sortedStandings()
	start, end, showSections := standingsWindow(rows, m.standingSelected, m.contentHeight()-5, m.standingView == standingsWildCard)
	lastConference := ""
	for i := start; i < end; i++ {
		r := rows[i]
		if showSections && r.Conference != lastConference {
			lines = append(lines, muted.Render("── "+strings.ToUpper(r.Conference)+" CONFERENCE ──"), "")
			lastConference = r.Conference
		}
		group := r.Division
		if m.standingView == standingsWildCard {
			switch {
			case r.WildcardSequence >= 1 && r.WildcardSequence <= 2:
				group = fmt.Sprintf("WILD CARD %d", r.WildcardSequence)
			case r.WildcardSequence > 2:
				group = ""
			default:
				group = fmt.Sprintf("%s %d", divisionCode(r.Division), r.DivisionSequence)
			}
		} else if m.standingView == standingsConference {
			group = r.Conference
		} else if m.standingView == standingsLeague {
			group = "League"
		}
		code := strings.ToUpper(group)
		if strings.EqualFold(group, "Metropolitan") {
			code = "METRO"
		}
		if len(code) > 11 {
			code = code[:11]
		}
		values := []string{code, r.TeamAbbrev.Default, fmt.Sprintf("%3d", r.GP), fmt.Sprintf("%3d", r.Wins), fmt.Sprintf("%3d", r.Losses), fmt.Sprintf("%3d", r.OTL), fmt.Sprintf("%4d", r.Points)}
		if m.width >= 80 {
			values = append(values, fmt.Sprintf("%5.3f", r.PointPct), fmt.Sprintf("%3d", r.RegulationWins), fmt.Sprintf("%3d", r.GoalsFor), fmt.Sprintf("%3d", r.GoalsAgainst))
		}
		if m.width >= 100 {
			values = append(values, fmt.Sprintf("%+4d", r.GoalDiff), fmt.Sprintf("%s%d", r.StreakCode, r.StreakCount))
		}
		line := columnsRight(starts, 2, lastNumber, values...)
		lines = append(lines, m.row(line, i == m.standingSelected, false))
	}
	return lines
}
func divisionCode(name string) string {
	switch {
	case strings.EqualFold(name, "Atlantic"):
		return "ATL"
	case strings.EqualFold(name, "Metropolitan"):
		return "MET"
	case strings.EqualFold(name, "Central"):
		return "CEN"
	case strings.EqualFold(name, "Pacific"):
		return "PAC"
	default:
		return strings.ToUpper(name)
	}
}
func (m Model) teamView() []string {
	abbr := m.teamAbbr
	name := abbr
	for _, r := range m.standings {
		if r.TeamAbbrev.Default == abbr {
			name = r.TeamName.Default
			break
		}
	}
	lines := []string{title.Render(strings.ToUpper(name))}
	for _, r := range m.standings {
		if r.TeamAbbrev.Default == abbr {
			lines = append(lines, fmt.Sprintf("%d-%d-%d · %d pts", r.Wins, r.Losses, r.OTL, r.Points))
			break
		}
	}
	mainOptions := optionTabs([]string{"Recent", "Schedule", "Roster", "Stats"}, m.teamTab)
	if m.teamTab == 3 && m.teamStatsFocus == 1 {
		mainOptions = "  " + mainOptions
	} else {
		mainOptions = "› " + mainOptions
	}
	lines = append(lines, muted.Render(mainOptions), "")
	var rows []string
	switch m.teamTab {
	case 0, 1:
		games := m.teamGames
		section := "Recent / upcoming"
		if m.teamTab == 1 {
			games, section = m.teamAllGames, "Season schedule"
		}
		lines = append(lines, muted.Render(section), "")
		for _, g := range games {
			rows = append(rows, m.teamGameLine(g))
		}
	case 2:
		starts := []int{0, 6, 32, 38, 46, 54}
		lines = append(lines, muted.Render("Roster · "+m.injurySource()), m.rowHeader(columnsRight(starts, 5, 5, "NO", "PLAYER", "POS", "S/C", "INJ", "P/SV%")), "")
		points := map[int]int{}
		for _, p := range m.teamStats.Skaters {
			points[p.ID] = p.Points
		}
		savePct := map[int]float64{}
		for _, p := range m.teamStats.Goalies {
			savePct[p.ID] = p.SavePct
		}
		goalieIDs := map[int]bool{}
		for _, p := range m.teamRoster.Goalies {
			goalieIDs[p.ID] = true
		}
		for _, p := range m.sortedRosterPlayers() {
			metric := ""
			if goalieIDs[p.ID] {
				if v, ok := savePct[p.ID]; ok {
					metric = fmt.Sprintf("%.3f", v)
				}
			} else if v, ok := points[p.ID]; ok {
				metric = fmt.Sprint(v)
			}
			rows = append(rows, columnsRight(starts, 5, 5, fmt.Sprint(p.Number), p.FullName(), p.Position, p.Shoots, m.injuryStatus(abbr, p.FullName()), metric))
		}
	case 3:
		options := optionTabs([]string{"Skaters", "Goalies"}, 0)
		if m.teamGoalies {
			options = optionTabs([]string{"Skaters", "Goalies"}, 1)
		}
		if m.teamStatsFocus == 1 {
			options = "› " + options
		} else {
			options = "  " + options
		}
		if m.teamGoalies {
			lines = append(lines, muted.Render(options), m.rowHeader(columnsRight([]int{0, 28, 34, 39, 44, 49, 56}, 1, 6, "GOALIE", "GP", "W", "L", "OTL", "GAA", "SV%")), "")
			for _, p := range m.sortedTeamGoalies() {
				rows = append(rows, columnsRight([]int{0, 28, 34, 39, 44, 49, 56}, 1, 6, p.FullName(), fmt.Sprint(p.GP), fmt.Sprint(p.Wins), fmt.Sprint(p.Losses), fmt.Sprint(p.OTL), fmt.Sprintf("%.2f", p.GAA), fmt.Sprintf("%.3f", p.SavePct)))
			}
		} else {
			lines = append(lines, muted.Render(options), m.rowHeader(columnsRight([]int{0, 28, 34, 39, 44, 49, 54}, 1, 6, "SKATER", "GP", "G", "A", "P", "+/-", "S")), "")
			for _, p := range m.sortedTeamSkaters() {
				rows = append(rows, columnsRight([]int{0, 28, 34, 39, 44, 49, 54}, 1, 6, p.FullName(), fmt.Sprint(p.GP), fmt.Sprint(p.Goals), fmt.Sprint(p.Assists), fmt.Sprint(p.Points), fmt.Sprintf("%+d", p.PlusMinus), fmt.Sprint(p.Shots)))
			}
		}
	}
	if len(rows) == 0 && !m.busy {
		detail := ""
		if m.teamTab == 2 {
			detail = "roster"
		}
		if m.teamTab == 3 {
			detail = "stats"
		}
		if m.pending[detail] {
			lines = append(lines, "Loading…")
		} else if m.detailErrors[detail] != "" {
			lines = append(lines, "Unavailable · press r to retry")
		} else {
			lines = append(lines, "No data available")
		}
	}
	visible := max(1, m.contentHeight()-len(lines))
	start := windowStart(m.teamSelected, len(rows), visible)
	for i := start; i < min(len(rows), start+visible); i++ {
		lines = append(lines, m.row(rows[i], i == m.teamSelected, false))
	}
	return lines
}
func (m Model) teamGameLine(g nhl.Game) string {
	opp, at := g.Opponent(m.teamAbbr)
	scoreText := ""
	if g.Final() || g.Live() {
		our, their := g.Home, g.Away
		if g.Away.Abbrev == m.teamAbbr {
			our, their = g.Away, g.Home
		}
		scoreText = g.Score(our) + "–" + g.Score(their)
	}
	status := g.Status()
	if m.teamTab == 1 {
		switch g.GameType {
		case 1:
			status += " · PRE"
		case 3:
			status += " · PLAYOFF"
		}
	}
	return columns([]int{0, 9, 14, 23, scheduleResultStart(m.width) + 2}, g.StartUTC.In(nhl.Eastern()).Format("Jan 02"), at, opp, scoreText, status)
}
func (m Model) playerView() []string {
	p := m.player
	name := p.FullName()
	if name == "" {
		name = "Player"
	}
	lines := []string{title.Render(fmt.Sprintf("#%d %s  ·  %s %s", p.Number, name, p.Team, p.Position))}
	if m.busy && p.ID == 0 {
		return []string{title.Render("Player"), muted.Render("Loading player…"), ""}
	}
	if status := m.injuryStatus(p.Team, p.FullName()); status != "" {
		lines = append(lines, live.Render("Injury: "+status))
	}
	lines = append(lines, muted.Render(m.injurySource()))
	if p.BirthDate != "" {
		lines = append(lines, muted.Render(fmt.Sprintf("Born %s · %s, %s · Shoots %s", p.BirthDate, p.BirthCity.Default, p.BirthCountry, p.Shoots)))
	}
	season := p.Featured.Regular.Season
	career := p.Featured.Regular.Career
	seasonLabel := "SEASON"
	if p.Featured.Season > 0 {
		seasonLabel = fmt.Sprintf("%d-%02d", p.Featured.Season/10000, p.Featured.Season%100)
	}
	if p.Position == "G" {
		starts := []int{0, 12, 18, 24, 30, 36, 44, 52}
		lines = append(lines, "", columnsRight(starts, 1, len(starts)-1, "", "GP", "W", "L", "OTL", "GAA", "SV%", "SO"),
			columnsRight(starts, 1, len(starts)-1, seasonLabel, fmt.Sprint(season.GP), fmt.Sprint(season.Wins), fmt.Sprint(season.Losses), fmt.Sprint(season.OTL), fmt.Sprintf("%.2f", season.GAA), fmt.Sprintf("%.3f", season.SavePct), fmt.Sprint(season.Shutouts)),
			columnsRight(starts, 1, len(starts)-1, "CAREER", fmt.Sprint(career.GP), fmt.Sprint(career.Wins), fmt.Sprint(career.Losses), fmt.Sprint(career.OTL), fmt.Sprintf("%.2f", career.GAA), fmt.Sprintf("%.3f", career.SavePct), fmt.Sprint(career.Shutouts)),
			"", muted.Render("Game log"), m.rowHeader(columnsRight([]int{0, 9, 18, 24, 30, 37, 46}, 3, 5, "DATE", "OPP", "DEC", "SA", "GA", "SV%", "TOI")), "")
	} else {
		starts := []int{0, 12, 18, 24, 30, 36, 44}
		lines = append(lines, "", columnsRight(starts, 1, len(starts)-1, "", "GP", "G", "A", "P", "+/-", "S"),
			columnsRight(starts, 1, len(starts)-1, seasonLabel, fmt.Sprint(season.GP), fmt.Sprint(season.Goals), fmt.Sprint(season.Assists), fmt.Sprint(season.Points), fmt.Sprintf("%+d", season.PlusMinus), fmt.Sprint(season.Shots)),
			columnsRight(starts, 1, len(starts)-1, "CAREER", fmt.Sprint(career.GP), fmt.Sprint(career.Goals), fmt.Sprint(career.Assists), fmt.Sprint(career.Points), fmt.Sprintf("%+d", career.PlusMinus), fmt.Sprint(career.Shots)),
			"", muted.Render("Game log"), m.rowHeader(columnsRight([]int{0, 9, 18, 24, 30, 36, 42, 49}, 2, 6, "DATE", "OPP", "G", "A", "P", "+/-", "S", "TOI")), "")
	}
	if len(m.playerLog) == 0 && !m.busy {
		if m.pending["player log"] {
			lines = append(lines, "Loading game log…")
		} else if m.detailErrors["player log"] != "" {
			lines = append(lines, "Game log unavailable · press r to retry")
		} else {
			lines = append(lines, "No games available")
		}
	}
	visible := max(1, m.contentHeight()-len(lines))
	start := windowStart(m.playerSelected, len(m.playerLog), visible)
	for i := start; i < min(len(m.playerLog), start+visible); i++ {
		g := m.playerLog[i]
		date := g.Date
		if t, err := time.Parse("2006-01-02", g.Date); err == nil {
			date = t.Format("Jan 02")
		}
		opp := "vs " + g.Opponent
		if g.HomeRoad == "R" {
			opp = "@ " + g.Opponent
		}
		line := ""
		if p.Position == "G" {
			line = columnsRight([]int{0, 9, 18, 24, 30, 37, 46}, 3, 5, date, opp, g.Decision, fmt.Sprint(g.ShotsAgainst), fmt.Sprint(g.GoalsAgainst), fmt.Sprintf("%.3f", g.SavePct), g.TOI)
		} else {
			line = columnsRight([]int{0, 9, 18, 24, 30, 36, 42, 49}, 2, 6, date, opp, fmt.Sprint(g.Goals), fmt.Sprint(g.Assists), fmt.Sprint(g.Points), fmt.Sprintf("%+d", g.PlusMinus), fmt.Sprint(g.Shots), g.TOI)
		}
		lines = append(lines, m.row(line, i == m.playerSelected, false))
	}
	return lines
}
func (m Model) teamsView() []string {
	lines := []string{title.Render("NHL Stats Vim  ·  Teams"), muted.Render("Choose a club to open its schedule, roster, and stats"), "", m.rowHeader(columnsRight([]int{0, 28, 36, 48}, 3, 3, "TEAM", "CODE", "DIVISION", "PTS")), ""}
	rows := m.sortedTeams()
	if len(rows) == 0 && !m.busy {
		lines = append(lines, "No teams available")
	}
	visible := max(1, m.contentHeight()-len(lines))
	start := windowStart(m.teamsSelected, len(rows), visible)
	for i := start; i < min(len(rows), start+visible); i++ {
		r := rows[i]
		division := strings.ToUpper(r.Division)
		if r.Division == "Metropolitan" {
			division = "METRO"
		}
		lines = append(lines, m.row(columnsRight([]int{0, 28, 36, 48}, 3, 3, r.TeamName.Default, r.TeamAbbrev.Default, division, fmt.Sprint(r.Points)), i == m.teamsSelected, false))
	}
	return lines
}
func (m Model) leadersView() []string {
	typeOptions := []string{"Skaters", "Goalies"}
	metricOptions := []string{"Points", "Goals", "Assists", "Shots"}
	if m.leadersGoalies {
		metricOptions = []string{"Save %", "Wins", "GAA"}
	}
	typeIndex := 0
	if m.leadersGoalies {
		typeIndex = 1
	}
	group := optionTabs(typeOptions, typeIndex)
	metrics := optionTabs(metricOptions, m.leadersMetric)
	if m.leadersFocus == 0 {
		group = "› " + group
		metrics = "  " + metrics
	} else {
		group = "  " + group
		metrics = "› " + metrics
	}
	scope := "Top 100 · regular season · tab focus · h/l choice"
	if m.leadersGoalies {
		if m.goalieQualified {
			scope += fmt.Sprintf(" · ≥%d GP", m.goalieMinimumGames())
		} else {
			scope += " · all goalies"
		}
		scope += " · u qualify"
	}
	lines := []string{
		title.Render("League leaders"),
		muted.Render(group),
		muted.Render(metrics),
		muted.Render(scope),
		"",
	}
	starts := []int{0, 5, 30, 37, 43, 49, 55, 61, 68, 74}
	if m.leadersGoalies {
		lines = append(lines, m.rowHeader(columnsRight(starts, 3, 8, "#", "GOALIE", "TEAM", "GP", "W", "L", "OTL", "GAA", "SV%", "SO")))
	} else {
		lines = append(lines, m.rowHeader(columnsRight(starts, 3, 8, "#", "SKATER", "TEAM", "GP", "G", "A", "P", "S", "+/-", "POS")))
	}
	lines = append(lines, "")
	count := m.leadersCount()
	if count == 0 && !m.busy {
		if m.leadersGoalies && m.goalieQualified {
			lines = append(lines, "No goalies meet the games-played threshold")
		} else {
			lines = append(lines, "No leaders available")
		}
	}
	visible := max(1, m.contentHeight()-len(lines))
	start := windowStart(m.leadersSelected, count, visible)
	for i := start; i < min(count, start+visible); i++ {
		var row string
		if m.leadersGoalies {
			p := m.visibleGoalieLeaders()[i]
			row = columnsRight(starts, 3, 8, fmt.Sprint(i+1), p.Name, p.Team, fmt.Sprint(p.GP), fmt.Sprint(p.Wins), fmt.Sprint(p.Losses), fmt.Sprint(p.OTL), fmt.Sprintf("%.2f", p.GAA), fmt.Sprintf("%.3f", p.SavePct), fmt.Sprint(p.Shutouts))
		} else {
			p := m.skaterLeaders[i]
			row = columnsRight(starts, 3, 8, fmt.Sprint(i+1), p.Name, p.Team, fmt.Sprint(p.GP), fmt.Sprint(p.Goals), fmt.Sprint(p.Assists), fmt.Sprint(p.Points), fmt.Sprint(p.Shots), fmt.Sprintf("%+d", p.PlusMinus), p.Position)
		}
		lines = append(lines, m.row(row, i == m.leadersSelected, false))
	}
	return lines
}
