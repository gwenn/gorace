package main

import (
	"fmt"
	"github.com/gwenn/gosqlite"
	"time"
)

const (
	TIME_LOG_CREATE = `INSERT INTO time_log VALUES (?, ?)`
	//	TIME_LOG_UPDATE  = `UPDATE time_log SET team_id = ?, time = ? WHERE team_id = ? AND time = ?`
	//	TIME_LOG_DELETE  = `DELETE FROM time_log WHERE team_id = ? AND time = ?`
	TIME_LOG_BY_TEAM = `SELECT time FROM time_log WHERE team_id = ? ORDER BY time desc`
	TIME_LOG_QUERY   = `SELECT tl.team_id, tl.time
FROM time_log tl
ORDER BY tl.time desc, tl.team_id asc LIMIT ?`
	TIME_LOG_DEFAULT_LIMIT = 100
	TIME_FORMAT            = "15:04:05"
)

type TimeLog struct {
	Team    *Team
	Time    string // "HH:MM:SS"
	LapTime string // 3m5s
}

func saveTimeLogs(db *sqlite.Conn, teamIds []int, time string) (err error) {
	tracef("Saving time logs (%v, %s)\n", teamIds, time)
	s, err := db.Prepare(TIME_LOG_CREATE)
	if err != nil {
		return
	}
	defer s.Finalize()
	err = db.Begin()
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			db.Rollback()
		}
		err = db.Commit()
	}()
	var n int
	for _, teamId := range teamIds {
		n, err = s.ExecDml(teamId, time)
		if err != nil {
			return
		} else if n != 1 {
			err = fmt.Errorf("No change while saving time log (%d, %s)", teamId, time)
			return
		}
	}
	tracef("Time logs (%v, %s) saved\n", teamIds, time)
	return
}

func loadTimeLogsByTeam(db *sqlite.Conn, teamId int) ([]TimeLog, error) {
	trace("Loading time logs...")
	s, err := db.Prepare(TIME_LOG_BY_TEAM, teamId)
	if err != nil {
		return nil, err
	}
	defer s.Finalize()
	var timeLogs []TimeLog = make([]TimeLog, 0, 100)
	err = withTeamCache(db, func(cache *TeamCache) error {
		team := cache.get(teamId)
		return s.Select(func(s *sqlite.Stmt) (err error) {
			tl := TimeLog{}
			if err = s.Scan(&tl.Time); err != nil {
				return
			}
			tl.Team = team
			timeLogs = append(timeLogs, tl)
			return
		})
	})
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(timeLogs)-1; i++ {
		timeLogs[i].LapTime = lapTime(timeLogs[i+1].Time, timeLogs[i].Time)
	}
	tracef("Loaded %d time logs.\n", len(timeLogs))
	return timeLogs, nil
}

func lapTime(start, end string) (lt string) {
	startTime, err := time.Parse(TIME_FORMAT, start)
	if err != nil {
		warn("Invalid time %q\n", start)
		return
	}
	endTime, err := time.Parse(TIME_FORMAT, end)
	if err != nil {
		warn("Invalid time %q\n", start)
		return
	}
	lt = endTime.Sub(startTime).String()
	return
}

func loadTimeLogs(db *sqlite.Conn, limit int) ([]TimeLog, error) {
	trace("Loading time logs...")
	if limit <= 0 {
		limit = TIME_LOG_DEFAULT_LIMIT
	}
	s, err := db.Prepare(TIME_LOG_QUERY, limit)
	if err != nil {
		return nil, err
	}
	defer s.Finalize()
	var timeLogs []TimeLog = make([]TimeLog, 0, 100)
	err = withTeamCache(db, func(cache *TeamCache) error {
		return s.Select(func(s *sqlite.Stmt) (err error) {
			tl := TimeLog{}
			var teamId int
			if err = s.Scan(&teamId, &tl.Time); err != nil {
				return
			}
			tl.Team = cache.get(teamId)
			timeLogs = append(timeLogs, tl)
			return
		})
	})
	if err != nil {
		return nil, err
	}
	tracef("Loaded %d time logs.\n", len(timeLogs))
	return timeLogs, nil
}
