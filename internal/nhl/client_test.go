package nhl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestFixtures(t *testing.T) {
	// These small, invented payloads exercise the response shapes without
	// redistributing captured NHL or ESPN responses.
	var s struct {
		Games []Game `json:"games"`
	}
	if err := json.Unmarshal(fixture(t, "score"), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Games) != 4 || !s.Games[0].Live() || s.Games[0].Status() != "2nd 13:29" ||
		!s.Games[1].Final() || s.Games[1].Status() != "FINAL" ||
		s.Games[2].Status() != "7:00 PM" || s.Games[3].Status() != "POSTPONED" {
		t.Fatal("schedule/live parse", s.Games)
	}
	var live, final Game
	if err := json.Unmarshal(fixture(t, "live"), &live); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture(t, "final"), &final); err != nil {
		t.Fatal(err)
	}
	if !live.Live() || !final.Final() || final.Away.Score == nil || final.Home.Score == nil {
		t.Fatal("game parse", live.State, final.State)
	}
	var p PBP
	if err := json.Unmarshal(fixture(t, "pbp"), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Plays) != 3 || p.Player(p.Plays[1].Details.Scorer) != "Alex Example" ||
		p.Plays[1].Details.HomeScore == nil || *p.Plays[1].Details.HomeScore != 1 {
		t.Fatal("PBP parse", p)
	}
	var st struct {
		Standings []Standing `json:"standings"`
	}
	if err := json.Unmarshal(fixture(t, "standings"), &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Standings) != 32 || st.Standings[0].TeamAbbrev.Default != "T01" ||
		st.Standings[16].Conference != "Western" {
		t.Fatal("standings parse")
	}
	var tm struct {
		Games []Game `json:"games"`
	}
	if err := json.Unmarshal(fixture(t, "team"), &tm); err != nil || len(tm.Games) == 0 {
		t.Fatal("team parse", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(code int, body []byte) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}
}
func TestCacheAndExpiry(t *testing.T) {
	var calls atomic.Int32
	c := NewClient(nil)
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.Header.Get("User-Agent") != userAgent {
			t.Error("user agent")
		}
		return reply(200, fixture(t, "score")), nil
	})
	clock := time.Now()
	c.now = func() time.Time { return clock }
	for i := 0; i < 2; i++ {
		if _, err := c.Today(context.Background(), "2026-10-05", false); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("cache miss")
	}
	clock = clock.Add(31 * time.Second)
	if _, err := c.Today(context.Background(), "2026-10-05", false); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("expiry miss")
	}
	if _, err := c.Today(context.Background(), "2026-10-05", true); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatal("manual refresh failed")
	}
}
func TestCacheBoundedAcrossDates(t *testing.T) {
	c := NewClient(nil)
	c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return reply(200, []byte(`{"games":[]}`)), nil })
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < maxCacheEntries+10; i++ {
		if _, err := c.Today(context.Background(), start.AddDate(0, 0, i).Format("2006-01-02"), false); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.cache) != maxCacheEntries {
		t.Fatalf("cache holds %d entries, want at most %d", len(c.cache), maxCacheEntries)
	}
}
func TestCacheBoundedByBytes(t *testing.T) {
	body := []byte(`{"data":"` + strings.Repeat("a", 1<<20) + `"}`)
	c := NewClient(nil)
	c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return reply(200, body), nil })
	for i := 0; i < 20; i++ {
		var result struct {
			Data string `json:"data"`
		}
		if err := c.get(context.Background(), fmt.Sprintf("/cache-test/%d", i), time.Hour, false, &result); err != nil {
			t.Fatal(err)
		}
	}
	total := 0
	for _, item := range c.cache {
		total += len(item.data)
	}
	if total > maxCacheBytes {
		t.Fatalf("cache grew to %d bytes", total)
	}
}

func TestLiveScoreExpiresAfterTenSeconds(t *testing.T) {
	var calls atomic.Int32
	c := NewClient(nil)
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return reply(200, fixture(t, "score")), nil
	})
	clock := time.Now()
	c.now = func() time.Time { return clock }
	if _, err := c.Today(context.Background(), "2026-10-05", false); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(11 * time.Second)
	if _, err := c.Today(context.Background(), "2026-10-05", false); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("live score cache did not expire", calls.Load())
	}
}
func TestTimeoutAndErrors(t *testing.T) {
	c := NewClient(nil)
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Today(ctx, "2026-10-05", false)
	if err == nil || !strings.Contains(err.Error(), "/score/2026-10-05") {
		t.Fatal(err)
	}
	var calls atomic.Int32
	c = NewClient(nil)
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) < 2 {
			return reply(503, nil), nil
		}
		return reply(200, fixture(t, "score")), nil
	})
	if _, err = c.Today(context.Background(), "2026-10-05", false); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("retry count", calls.Load())
	}
}

func TestGamecenterDetailShapes(t *testing.T) {
	landing := []byte(`{"tvBroadcasts":[{"network":"MSG","market":"H"}],"summary":{"scoring":[{"periodDescriptor":{"number":1,"periodType":"REG"},"goals":[{"playerId":7,"name":{"default":"A. Scorer"},"teamAbbrev":{"default":"NJD"},"timeInPeriod":"04:12","assists":[{"name":{"default":"B. Helper"}}]}]}],"penalties":[{"periodDescriptor":{"number":1,"periodType":"REG"},"penalties":[{"timeInPeriod":"08:00","duration":2,"descKey":"tripping","teamAbbrev":{"default":"NJD"},"committedByPlayer":{"firstName":{"default":"C."},"lastName":{"default":"Skater"}}}]}],"threeStars":[{"star":1,"playerId":7,"teamAbbrev":"NJD","name":{"default":"A. Scorer"}}]}}`)
	box := []byte(`{"id":42,"awayTeam":{"abbrev":"NJD"},"homeTeam":{"abbrev":"NYR"},"playerByGameStats":{"awayTeam":{"forwards":[{"playerId":7,"name":{"default":"A. Scorer"},"goals":1,"assists":1,"points":2,"sog":4,"toi":"18:20"}],"goalies":[{"playerId":8,"name":{"default":"D. Goalie"},"saves":29,"shotsAgainst":30,"goalsAgainst":1}]}}}`)
	c := NewClient(nil)
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/landing") {
			return reply(200, landing), nil
		}
		return reply(200, box), nil
	})
	s, err := c.Summary(context.Background(), 42, time.Minute, false)
	if err != nil || len(s.Summary.Scoring) != 1 || s.Summary.Scoring[0].Goals[0].Name.Default != "A. Scorer" || s.Summary.Penalties[0].Penalties[0].Description != "tripping" || s.Summary.ThreeStars[0].PlayerID != 7 || s.Broadcasts[0].Network != "MSG" {
		t.Fatalf("summary decode: %+v %v", s, err)
	}
	g, err := c.Boxscore(context.Background(), 42, time.Minute, false)
	if err != nil || g.PlayerStats.Away.Forwards[0].Points != 2 || g.PlayerStats.Away.Goalies[0].Saves != 29 {
		t.Fatalf("box score decode: %+v %v", g, err)
	}
}

func TestTeamPlayerAndLeadersEndpoints(t *testing.T) {
	c := NewClient(nil)
	var goalieSorts []string
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/roster/"):
			return reply(200, []byte(`{"forwards":[{"id":7,"firstName":{"default":"A."},"lastName":{"default":"Skater"},"positionCode":"C"}]}`)), nil
		case strings.HasPrefix(r.URL.Path, "/v1/club-stats/"):
			return reply(200, []byte(`{"skaters":[{"playerId":7,"firstName":{"default":"A."},"lastName":{"default":"Skater"},"points":2}],"goalies":[{"playerId":8,"firstName":{"default":"B."},"lastName":{"default":"Goalie"},"savePercentage":0.925}]}`)), nil
		case strings.HasSuffix(r.URL.Path, "/landing"):
			return reply(200, []byte(`{"playerId":7,"firstName":{"default":"A."},"lastName":{"default":"Skater"},"featuredStats":{"regularSeason":{"subSeason":{"points":2}}}}`)), nil
		case strings.Contains(r.URL.Path, "/game-log/"):
			return reply(200, []byte(`{"gameLog":[{"gameId":42,"opponentAbbrev":"NYR","points":2}]}`)), nil
		case strings.Contains(r.URL.Path, "/stats/rest/en/"):
			if r.URL.Query().Get("cayenneExp") != "seasonId=20262027 and gameTypeId=2" || r.URL.Query().Get("limit") != "100" {
				t.Errorf("wrong stats query: %s", r.URL.RawQuery)
			}
			if strings.Contains(r.URL.Path, "/goalie/") {
				goalieSorts = append(goalieSorts, r.URL.Query().Get("sort"))
				return reply(200, []byte(`{"data":[{"playerId":8,"goalieFullName":"B. Goalie","savePct":0.925}]}`)), nil
			}
			return reply(200, []byte(`{"data":[{"playerId":7,"skaterFullName":"A. Skater","points":2}]}`)), nil
		}
		return reply(404, nil), nil
	})
	roster, err := c.Roster(context.Background(), "NJD", 20262027, false)
	if err != nil || roster.Players()[0].FullName() != "A. Skater" {
		t.Fatalf("roster: %+v %v", roster, err)
	}
	stats, err := c.ClubStats(context.Background(), "NJD", 20262027, false)
	if err != nil || stats.Skaters[0].Points != 2 || stats.Goalies[0].SavePct != 0.925 {
		t.Fatalf("club stats: %+v %v", stats, err)
	}
	player, err := c.Player(context.Background(), 7, false)
	if err != nil || player.Featured.Regular.Season.Points != 2 {
		t.Fatalf("player: %+v %v", player, err)
	}
	log, err := c.PlayerLog(context.Background(), 7, 20262027, false)
	if err != nil || log[0].GameID != 42 {
		t.Fatalf("game log: %+v %v", log, err)
	}
	skaters, _, err := c.Leaders(context.Background(), 20262027, false, 0, false)
	if err != nil || skaters[0].ID != 7 {
		t.Fatalf("skater leaders: %+v %v", skaters, err)
	}
	_, goalies, err := c.Leaders(context.Background(), 20262027, true, 2, false)
	if err != nil || goalies[0].ID != 8 {
		t.Fatalf("goalie leaders: %+v %v", goalies, err)
	}
	_, _, err = c.Leaders(context.Background(), 20262027, true, 0, false)
	if err != nil || len(goalieSorts) != 2 || !strings.Contains(goalieSorts[0], `"goalsAgainstAverage"`) || !strings.Contains(goalieSorts[1], `"savePct"`) {
		t.Fatalf("goalie metric order: %v, %v", goalieSorts, err)
	}
}

func TestInjuryFeedStatusesAndTeamCodes(t *testing.T) {
	report := []byte(`{"injuries":[{"id":"11","injuries":[{"status":"Injured Reserve","athlete":{"displayName":"A. Skater"},"details":{"fantasyStatus":{"abbreviation":"IR"}}},{"status":"Day-To-Day","athlete":{"displayName":"B. Skater"}},{"status":"Injured Reserve","athlete":{"displayName":"C. Skater"},"details":{"fantasyStatus":{"abbreviation":"IR-LT"}}},{"status":"Suspension","athlete":{"displayName":"D. Skater"}}]},{"id":"129764","injuries":[{"status":"Out","athlete":{"displayName":"E. Skater"}}]}]}`)
	teams := []byte(`{"sports":[{"leagues":[{"teams":[{"team":{"id":"11","abbreviation":"NJ"}},{"team":{"id":"129764","abbreviation":"UTAH"}}]}]}]}`)
	var calls int
	c := NewClient(nil)
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "site.api.espn.com" {
			t.Errorf("injury request went to %s", r.URL.Host)
		}
		if strings.HasSuffix(r.URL.Path, "/injuries") {
			return reply(200, report), nil
		}
		return reply(200, teams), nil
	})
	want := []Injury{{"NJD", "A. Skater", "IR"}, {"NJD", "B. Skater", "DTD"}, {"NJD", "C. Skater", "LTIR"}, {"UTA", "E. Skater", "OUT"}}
	for i := 0; i < 2; i++ {
		got, err := c.Injuries(context.Background(), false)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("injuries = %+v, %v; want %+v", got, err, want)
		}
	}
	if calls != 2 {
		t.Fatalf("injury feed was not cached: %d calls", calls)
	}
}
