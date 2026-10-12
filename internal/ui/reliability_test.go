package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Malvore/nhl-stats-vim/internal/nhl"
	tea "github.com/charmbracelet/bubbletea"
)

func TestReviewScheduledGameKeepsPolling(t *testing.T) {
	m := model(nil)
	m.screen = Game
	m.game = sampleGames()[2]
	_, cmd := m.Update(result{kind: Game, target: m.game.ID, game: m.game})
	if cmd == nil {
		t.Fatal("scheduled Game result emits no refresh timer; it cannot notice puck drop")
	}
}

func TestReviewRejectsOlderSameScreenRequest(t *testing.T) {
	m := model(sampleGames())
	m.today, m.busy = sampleGames(), false
	m.generation = 4
	next, _ := m.Update(result{kind: Today, generation: 3, day: m.day(), games: nil})
	m = next.(Model)
	if len(m.today) != 3 {
		t.Fatal("older response replaced a newer Today result")
	}
}

func TestReviewPrimaryTeamResultBeforeDetails(t *testing.T) {
	m := model(sampleGames())
	m.screen, m.teamAbbr, m.busy = Team, "NJD", true
	m.generation = 2
	primary := result{kind: Team, generation: 2, teamAbbr: "NJD", games: sampleGames()}
	next, cmd := m.Update(primary)
	m = next.(Model)
	if m.busy || cmd == nil || !m.pending["roster"] || !m.pending["injuries"] {
		t.Fatal("team schedule did not render while details load")
	}
	m.injuries = []nhl.Injury{{Team: "NJD", Player: "A Player", Status: "IR"}}
	m.injuryAvailable = true
	m.injuryUpdated = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	next, _ = m.Update(result{kind: Team, generation: 2, teamAbbr: "NJD", detail: "injuries", err: errors.New("offline")})
	m = next.(Model)
	if len(m.injuries) != 1 || !strings.Contains(plain(m.View()), "Injuries unavailable") || !strings.Contains(m.injurySource(), "last successful") {
		t.Fatal("detail failure lost old injury data or hid its stale status")
	}
}

func TestReviewGoalieQualificationAndFocusedOptions(t *testing.T) {
	m := model(nil)
	m.screen, m.busy, m.leadersGoalies = Leaders, false, true
	m.goalieLeaders = []nhl.LeaderGoalie{{ID: 1, GP: 1, SavePct: 1}, {ID: 2, GP: 4, SavePct: .95}}
	m = key(m, "u")
	if !m.goalieQualified || m.leadersCount() != 1 || m.leadersPlayerID() != 2 || !strings.Contains(plain(m.View()), "≥2 GP") {
		t.Fatal("goalie qualification changed ordering or hid its threshold")
	}
	m = special(m, tea.KeyTab)
	m = key(m, "l")
	if m.leadersFocus != 1 || m.leadersMetric != 1 || !m.leadersGoalies {
		t.Fatal("metric row did not use h/l after focus change")
	}
	m.screen, m.teamTab, m.teamStatsFocus = Team, 3, 0
	m = special(m, tea.KeyTab)
	m = key(m, "l")
	if m.teamTab != 3 || !m.teamGoalies || m.teamStatsFocus != 1 {
		t.Fatal("team stats option did not use h/l")
	}
}

func TestReviewManpowerDistinguishesExtraAttacker(t *testing.T) {
	if got := manpowerLabel("1451", "NJD", "BOS"); got != "BOS power play" {
		t.Fatal(got)
	}
	if got := manpowerLabel("1560", "NJD", "BOS"); got != "BOS extra attacker" {
		t.Fatal(got)
	}
	if got := manpowerLabel("1551", "NJD", "BOS"); got != "" {
		t.Fatal(got)
	}
}

func TestReviewLeaderPositionHeaderFits80(t *testing.T) {
	m := model(nil)
	m.screen, m.busy = Leaders, false
	if !strings.Contains(plain(m.View()), "POS") {
		t.Fatal("position header is clipped at 80 columns")
	}
}

func TestReviewBackResumesPolling(t *testing.T) {
	m := model(sampleGames())
	m.today, m.busy = sampleGames(), false
	m = key(m, "a")
	next, cmd := m.Update(tick{})
	m = next.(Model)
	if cmd != nil {
		t.Fatal("unexpected timer while in Teams")
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.screen != Today {
		t.Fatal("did not return to Today")
	}
	if cmd == nil {
		t.Fatal("last timer was consumed in Teams; Back to Today emits neither a fetch nor a new timer")
	}
}

func TestReviewBackKeepsCurrentTerminalSize(t *testing.T) {
	m := model(sampleGames())
	m.today, m.busy = sampleGames(), false
	m = special(m, tea.KeyEnter)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = special(next.(Model), tea.KeyEsc)
	if m.width != 120 || m.height != 40 {
		t.Fatalf("Back restored obsolete terminal size %dx%d", m.width, m.height)
	}
}

func TestReviewRefreshFailureVisibleOnFullScreen(t *testing.T) {
	m := model(nil)
	for i := 0; i < 30; i++ {
		m.today = append(m.today, sampleGames()[0])
	}
	m.busy = false
	m.err = errors.New("offline")
	if !strings.Contains(plain(m.View()), "refresh failed") {
		t.Fatal("full Today list clips the refresh failure message entirely")
	}
}

func TestReviewRefreshKeepsSelectedPlayer(t *testing.T) {
	m := model(nil)
	m.screen, m.gameTab, m.busy = Game, 1, false
	m.game = sampleGames()[0]
	m.game.PlayerStats.Away.Forwards = []nhl.Skater{{ID: 1, Points: 2}, {ID: 2, Points: 1}}
	m.rebuildFeed()
	m = key(m, "j")
	updated := m.game
	updated.PlayerStats.Away.Forwards = []nhl.Skater{{ID: 1, Points: 2}, {ID: 2, Points: 3}}
	next, _ := m.Update(result{kind: Game, target: m.game.ID, game: updated})
	m = next.(Model)
	if got := m.boxPlayerIDs[m.boxSelected]; got != 2 {
		t.Fatalf("refresh moved selection from player 2 to player %d", got)
	}
}

func TestReviewBoxTabKeepsCursorVisible(t *testing.T) {
	m := model(nil)
	m.screen, m.gameTab, m.busy = Game, 1, false
	m.game = sampleGames()[0]
	for i := 1; i <= 40; i++ {
		m.game.PlayerStats.Away.Forwards = append(m.game.PlayerStats.Away.Forwards, nhl.Skater{ID: i})
	}
	m.rebuildFeed()
	m = key(m, "G")
	m = key(m, "l")
	m = key(m, "h")
	if !strings.Contains(plain(m.View()), "›") {
		t.Fatal("returning to Box Score resets scroll to top while selected player stays at bottom; cursor disappears")
	}
}

func TestReviewCoalescedMovement(t *testing.T) {
	m := model(nil)
	m.today, m.busy = sampleGames(), false
	m = key(m, "jj")
	if m.selected != 2 {
		t.Fatalf("coalesced jj moved to row %d instead of row 2", m.selected)
	}
}
