package main

import (
	"fmt"
	"github.com/gwenn/gosqlite"
)

const (
	RACE_QUERY  = `SELECT time FROM start_time`
	RACE_UPDATE = `UPDATE start_time SET time = ?`
)

type Race struct {
	StartTime string // "HH:MM:SS"
}

func loadRace(db *sqlite.Conn) (*Race, error) {
	trace("Loading race...")
	s, err := db.Prepare(RACE_QUERY)
	if err != nil {
		return nil, err
	}
	defer s.Finalize()
	var r *Race = &Race{}
	_, err = s.SelectOneRow(&r.StartTime)
	if err != nil {
		return nil, err
	}
	trace("Loaded race.")
	return r, nil
}

func saveRace(db *sqlite.Conn, time string) (err error) {
	tracef("Saving race (%s)\n", time)
	s, err := db.Prepare(RACE_UPDATE)
	if err != nil {
		return
	}
	defer s.Finalize()
	n, err := s.ExecDml(time)
	if err != nil {
		return
	} else if n != 1 {
		err = fmt.Errorf("No change while saving race (%s)\n", time)
		return
	}
	tracef("Race (%s) saved\n", time)
	return
}
