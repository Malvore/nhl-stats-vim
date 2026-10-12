package nhl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const base = "https://api-web.nhle.com/v1"
const userAgent = "nhl-stats-vim/1.0 (terminal hockey viewer; github.com/Malvore/nhl-stats-vim)"
const maxCacheEntries = 256
const maxCacheBytes = 12 << 20

type entry struct {
	data  []byte
	until time.Time
	used  time.Time
}
type Client struct {
	http  *http.Client
	base  string
	mu    sync.Mutex
	cache map[string]entry
	now   func() time.Time
	log   *slog.Logger
}

func NewClient(log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{http: &http.Client{Timeout: 8 * time.Second}, base: base, cache: make(map[string]entry), now: time.Now, log: log}
}
func (c *Client) get(ctx context.Context, path string, ttl time.Duration, force bool, out any) error {
	if !strings.HasPrefix(path, "/") {
		return errors.New("invalid NHL path")
	}
	c.mu.Lock()
	e, ok := c.cache[path]
	if !force && ok && c.now().Before(e.until) {
		e.used = c.now()
		c.cache[path] = e
		c.mu.Unlock()
		return json.Unmarshal(e.data, out)
	}
	c.mu.Unlock()
	var body []byte
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		endpoint := c.base + path
		if strings.HasPrefix(path, "/stats/") {
			endpoint = "https://api.nhle.com/stats/rest/en" + strings.TrimPrefix(path, "/stats")
		} else if strings.HasPrefix(path, "/espn/") {
			endpoint = "https://site.api.espn.com/apis/site/v2/sports/hockey/nhl" + strings.TrimPrefix(path, "/espn")
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if reqErr != nil {
			return fmt.Errorf("NHL %s: %w", path, reqErr)
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		began := time.Now()
		resp, doErr := c.http.Do(req)
		if doErr != nil {
			err = doErr
		} else {
			body, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			resp.Body.Close()
			if err == nil && resp.StatusCode == http.StatusOK {
				if !json.Valid(body) {
					err = errors.New("invalid JSON")
				} else {
					c.log.Debug("NHL request", "path", path, "latency", time.Since(began))
					break
				}
			}
			if err == nil {
				err = fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			if resp.StatusCode != 429 && resp.StatusCode < 500 {
				break
			}
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("NHL %s: %w", path, ctx.Err())
			case <-timer.C:
			}
		}
	}
	if err != nil {
		c.log.Error("NHL request failed", "path", path, "error", err)
		return fmt.Errorf("NHL %s: %w", path, err)
	}
	if err = json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("NHL %s decode: %w", path, err)
	}
	c.mu.Lock()
	now := c.now()
	size := 0
	for key, cached := range c.cache {
		if !now.Before(cached.until) {
			delete(c.cache, key)
		} else {
			size += len(cached.data)
		}
	}
	if previous, exists := c.cache[path]; exists {
		size -= len(previous.data)
		delete(c.cache, path)
	}
	if len(body) > maxCacheBytes {
		c.mu.Unlock()
		return nil
	}
	for len(c.cache) >= maxCacheEntries || size+len(body) > maxCacheBytes {
		oldestKey := ""
		var oldest time.Time
		for key, cached := range c.cache {
			if oldestKey == "" || cached.used.Before(oldest) {
				oldestKey, oldest = key, cached.used
			}
		}
		if oldestKey == "" {
			break
		}
		size -= len(c.cache[oldestKey].data)
		delete(c.cache, oldestKey)
	}
	c.cache[path] = entry{data: body, until: now.Add(ttl), used: now}
	c.mu.Unlock()
	return nil
}
func validDate(day string) bool { _, err := time.Parse("2006-01-02", day); return err == nil }
func (c *Client) Today(ctx context.Context, day string, force bool) ([]Game, error) {
	if !validDate(day) {
		return nil, errors.New("invalid date")
	}
	var p struct {
		Games []Game `json:"games"`
	}
	err := c.get(ctx, "/score/"+day, 30*time.Second, force, &p)
	if err != nil {
		return nil, err
	}
	SortGames(p.Games)
	for _, g := range p.Games {
		if g.Live() {
			path := "/score/" + day
			c.mu.Lock()
			e := c.cache[path]
			liveUntil := c.now().Add(10 * time.Second)
			if liveUntil.Before(e.until) {
				e.until = liveUntil
				c.cache[path] = e
			}
			c.mu.Unlock()
			break
		}
	}
	return p.Games, nil
}
func (c *Client) Boxscore(ctx context.Context, id int, ttl time.Duration, force bool) (Game, error) {
	var g Game
	if id <= 0 {
		return g, errors.New("invalid game ID")
	}
	err := c.get(ctx, "/gamecenter/"+strconv.Itoa(id)+"/boxscore", ttl, force, &g)
	return g, err
}
func (c *Client) Plays(ctx context.Context, id int, ttl time.Duration, force bool) (PBP, error) {
	var p PBP
	if id <= 0 {
		return p, errors.New("invalid game ID")
	}
	err := c.get(ctx, "/gamecenter/"+strconv.Itoa(id)+"/play-by-play", ttl, force, &p)
	return p, err
}
func (c *Client) Summary(ctx context.Context, id int, ttl time.Duration, force bool) (GameSummary, error) {
	var summary GameSummary
	if id <= 0 {
		return summary, errors.New("invalid game ID")
	}
	err := c.get(ctx, "/gamecenter/"+strconv.Itoa(id)+"/landing", ttl, force, &summary)
	return summary, err
}
func (c *Client) Standings(ctx context.Context, day string, force bool) ([]Standing, error) {
	if !validDate(day) {
		return nil, errors.New("invalid date")
	}
	var p struct {
		Standings []Standing `json:"standings"`
	}
	err := c.get(ctx, "/standings/"+day, 5*time.Minute, force, &p)
	return p.Standings, err
}
func (c *Client) TeamSchedule(ctx context.Context, abbr string, season int, force bool) ([]Game, error) {
	if len(abbr) != 3 || strings.ToUpper(abbr) != abbr || season < 20002001 || season > 21002101 {
		return nil, errors.New("invalid team or season")
	}
	var p struct {
		Games []Game `json:"games"`
	}
	path := "/club-schedule-season/" + url.PathEscape(abbr) + "/" + strconv.Itoa(season)
	err := c.get(ctx, path, 5*time.Minute, force, &p)
	if err != nil {
		return nil, err
	}
	SortGames(p.Games)
	return p.Games, nil
}
func (c *Client) Roster(ctx context.Context, abbr string, season int, force bool) (Roster, error) {
	var roster Roster
	if !validTeamSeason(abbr, season) {
		return roster, errors.New("invalid team or season")
	}
	err := c.get(ctx, "/roster/"+abbr+"/"+strconv.Itoa(season), 15*time.Minute, force, &roster)
	return roster, err
}
func (c *Client) ClubStats(ctx context.Context, abbr string, season int, force bool) (ClubStats, error) {
	var stats ClubStats
	if !validTeamSeason(abbr, season) {
		return stats, errors.New("invalid team or season")
	}
	err := c.get(ctx, "/club-stats/"+abbr+"/"+strconv.Itoa(season)+"/2", 5*time.Minute, force, &stats)
	return stats, err
}

func (c *Client) Injuries(ctx context.Context, force bool) ([]Injury, error) {
	var report struct {
		Injuries []struct {
			ID       string `json:"id"`
			Injuries []struct {
				Status  string `json:"status"`
				Athlete struct {
					Name string `json:"displayName"`
				} `json:"athlete"`
				Details struct {
					FantasyStatus struct {
						Code string `json:"abbreviation"`
					} `json:"fantasyStatus"`
				} `json:"details"`
			} `json:"injuries"`
		} `json:"injuries"`
	}
	if err := c.get(ctx, "/espn/injuries", 10*time.Minute, force, &report); err != nil {
		return nil, err
	}
	var teams struct {
		Sports []struct {
			Leagues []struct {
				Teams []struct {
					Team struct {
						ID     string `json:"id"`
						Abbrev string `json:"abbreviation"`
					} `json:"team"`
				} `json:"teams"`
			} `json:"leagues"`
		} `json:"sports"`
	}
	if err := c.get(ctx, "/espn/teams", 24*time.Hour, force, &teams); err != nil {
		return nil, err
	}
	abbrevs := map[string]string{}
	for _, sport := range teams.Sports {
		for _, league := range sport.Leagues {
			for _, item := range league.Teams {
				abbr := item.Team.Abbrev
				switch abbr {
				case "LA":
					abbr = "LAK"
				case "NJ":
					abbr = "NJD"
				case "SJ":
					abbr = "SJS"
				case "TB":
					abbr = "TBL"
				case "UTAH":
					abbr = "UTA"
				}
				abbrevs[item.Team.ID] = abbr
			}
		}
	}
	var injuries []Injury
	for _, group := range report.Injuries {
		abbr := abbrevs[group.ID]
		if abbr == "" {
			continue
		}
		for _, item := range group.Injuries {
			status := injuryCode(item.Details.FantasyStatus.Code, item.Status)
			if status != "" && item.Athlete.Name != "" {
				injuries = append(injuries, Injury{Team: abbr, Player: item.Athlete.Name, Status: status})
			}
		}
	}
	return injuries, nil
}

func injuryCode(fantasy, status string) string {
	switch fantasy {
	case "IR-LT":
		return "LTIR"
	case "IR", "IR-NR":
		return "IR"
	}
	switch status {
	case "Day-To-Day":
		return "DTD"
	case "Out":
		return "OUT"
	}
	return ""
}

func (c *Client) Player(ctx context.Context, id int, force bool) (PlayerProfile, error) {
	var player PlayerProfile
	if id <= 0 {
		return player, errors.New("invalid player ID")
	}
	err := c.get(ctx, "/player/"+strconv.Itoa(id)+"/landing", 15*time.Minute, force, &player)
	return player, err
}
func (c *Client) PlayerLog(ctx context.Context, id, season int, force bool) ([]PlayerGame, error) {
	if id <= 0 || season < 20002001 || season > 21002101 {
		return nil, errors.New("invalid player or season")
	}
	var result struct {
		Games []PlayerGame `json:"gameLog"`
	}
	err := c.get(ctx, "/player/"+strconv.Itoa(id)+"/game-log/"+strconv.Itoa(season)+"/2", 5*time.Minute, force, &result)
	return result.Games, err
}
func validTeamSeason(abbr string, season int) bool {
	return len(abbr) == 3 && strings.ToUpper(abbr) == abbr && season >= 20002001 && season <= 21002101
}
func (c *Client) Leaders(ctx context.Context, season int, goalies bool, metric int, force bool) ([]LeaderSkater, []LeaderGoalie, error) {
	if season < 20002001 || season > 21002101 {
		return nil, nil, errors.New("invalid season")
	}
	resource := "skater"
	property := []string{"points", "goals", "assists", "shots"}[max(0, min(metric, 3))]
	direction := "DESC"
	if goalies {
		resource = "goalie"
		property = []string{"savePct", "wins", "goalsAgainstAverage"}[max(0, min(metric, 2))]
		if metric == 2 {
			direction = "ASC"
		}
	}
	query := url.Values{}
	query.Set("isAggregate", "false")
	query.Set("isGame", "false")
	query.Set("cayenneExp", fmt.Sprintf("seasonId=%d and gameTypeId=2", season))
	query.Set("limit", "100")
	query.Set("start", "0")
	query.Set("sort", fmt.Sprintf(`[{"property":"%s","direction":"%s"}]`, property, direction))
	path := "/stats/" + resource + "/summary?" + query.Encode()
	if goalies {
		var response struct {
			Data []LeaderGoalie `json:"data"`
		}
		err := c.get(ctx, path, 5*time.Minute, force, &response)
		return nil, response.Data, err
	}
	var response struct {
		Data []LeaderSkater `json:"data"`
	}
	err := c.get(ctx, path, 5*time.Minute, force, &response)
	return response.Data, nil, err
}
func Season(t time.Time) int {
	y := t.In(Eastern()).Year()
	if t.In(Eastern()).Month() < 7 {
		y--
	}
	return y*10000 + y + 1
}
