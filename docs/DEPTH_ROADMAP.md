# NHL information coverage

NHL Stats Vim is an interactive terminal application with optional SSH service. Its
current navigation keeps a selected row, lets Enter drill into games, teams, and players, and
restores the previous view with Esc. Tabs switch related panels in place.
Vim motions and the persistent footer apply throughout.

## Structured hockey information now covered

| NHL.com area | TUI view | NHL source |
| --- | --- | --- |
| Scores and schedule | Date browsing and game selection | `/v1/score/{date}` |
| Gamecenter | Summary, box score, all or key play-by-play events | `/v1/gamecenter/{id}/landing`, `/boxscore`, `/play-by-play` |
| Standings | Division, conference, league, wild card | `/v1/standings/{date}` |
| Teams | Recent games, season schedule, roster, skater and goalie stats | `/v1/club-schedule-season/{team}/{season}`, `/v1/roster/{team}/{season}`, `/v1/club-stats/{team}/{season}/2` |
| Players | Profile, season/career totals, current-season game log | `/v1/player/{id}/landing`, `/game-log/{season}/2` |
| League leaders | Top 100 by points, goals, assists, shots, wins, save percentage, or goals-against average | NHL stats `/stats/rest/en/{skater,goalie}/summary` |

The NHL web and stats endpoints are public but undocumented. Tests cover the
response shapes used by the TUI. Optional detail requests load separately;
the primary game or team view stays usable if a secondary feed fails, and the
UI labels unavailable details. NHL stats responses cap
the requested leaderboard at 100 rows; each metric is sorted by the server
before that limit is applied.

## Next research and implementation slices

1. Add search/filter prompts within long team, player, and event lists, plus
   season and game-type selectors. Keep focus and scroll position stable during
   refreshes.
2. Expand Gamecenter team comparisons and scoring context only after verifying
   a reliable source for each field. Cover overtime, shootout, postponed,
   preseason, and playoff response variants with fixtures.
3. Research NHL EDGE data availability and its terminal presentation. Identify
   which advanced figures are exposed in structured responses before adding a
   view. See [NHL EDGE](https://www.nhl.com/news/topic/nhl-edge/nhl-edge-site-new-look-has-advanced-statistics-for-everybody).
4. Research NHL.com recap, news, and highlight metadata. A terminal panel can
   index titles, summaries, and links where a stable feed exists; video playback
   needs a separate product decision. See [NHL Gamecenter](https://www.nhl.com/gamecenter/buf-vs-det/2026/09/24/2026010041/summary).

These additions belong in the `nhl-stats-vim` application. An SSH deployment can
run the app as a stateless service with its own image and key mounts.
