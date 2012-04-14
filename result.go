package main

import (
	"github.com/gwenn/gosqlite"
	"time"
)

const RESULT_QUERY = `SELECT
  (SELECT count(1) + 1 FROM result rt WHERE rt.nb_lap > r.nb_lap OR (rt.nb_lap = r.nb_lap AND rt.time < r.time)) AS rank,
  r.team_id,
  r.nb_lap,
  strftime('%s', r.time) - strftime('%s', start_time.time)
FROM result r, start_time
ORDER BY rank`

type Result struct {
	Rank  int
	Team  *Team
	NbLap int
	Time  string // 9h3m5s
}

func loadResults(db *sqlite.Conn) ([]Result, error) {
	trace("Loading results...")
	s, err := db.Prepare(RESULT_QUERY)
	if err != nil {
		return nil, err
	}
	defer s.Finalize()
	var results []Result = make([]Result, 0, 20)
	err = withTeamCache(db, func(cache *TeamCache) error {
		return s.Select(func(s *sqlite.Stmt) (err error) {
			r := Result{}
			var teamId int
			var seconds int64
			if err = s.Scan(&r.Rank, &teamId, &r.NbLap, &seconds); err != nil {
				return
			}
			r.Team = cache.get(teamId)
			r.Time = time.Duration(seconds * 1e9).String()
			results = append(results, r)
			return
		})
	})
	if err != nil {
		return nil, err
	}
	tracef("Loaded %d results.\n", len(results))
	return results, nil
}
