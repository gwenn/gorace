package main

import (
	"fmt"
	"github.com/gwenn/gosqlite"
	"sync"
)

const (
	TEAM_QUERY  = `SELECT id, number, name FROM team ORDER BY number`
	TEAM_INSERT = `INSERT INTO team (number, name) VALUES (?, ?)`
	TEAM_UPDATE = `UPDATE team SET number = ?, name = ? WHERE id = ?`
	TEAM_DELETE = `DELETE FROM team WHERE id = ?`
)

type Team struct {
	Id     int
	Number int
	Name   string
}

type TeamCache struct {
	m        sync.RWMutex
	teamById map[int]*Team // unsorted
	teams    []*Team       // sorted
}

var cache *TeamCache = &TeamCache{teamById: make(map[int]*Team)}
var notInCache *Team = &Team{Id: -1, Number: -1, Name: "Not in cache"}

func withTeamCache(db *sqlite.Conn, f func(*TeamCache) error) error {
	if cache.empty() {
		_, err := loadTeams(db)
		if err != nil {
			return err
		}
	}
	return f(cache)
}

func (c *TeamCache) empty() bool {
	c.m.RLock()
	defer c.m.RUnlock()
	return len(c.teams) == 0
}

func (c *TeamCache) get(id int) *Team {
	cache.m.RLock()
	defer cache.m.RUnlock()
	team := c.teamById[id]
	if team == nil {
		warn("Team %d not in cache", id)
		team = notInCache
	}
	return team
}

func loadTeams(db *sqlite.Conn) ([]*Team, error) {
	trace("Loading teams...")
	s, err := db.Prepare(TEAM_QUERY)
	if err != nil {
		return nil, err
	}
	defer s.Finalize()
	var teams []*Team = make([]*Team, 0, 20)
	err = s.Select(func(s *sqlite.Stmt) (err error) {
		t := &Team{}
		if err = s.Scan(&t.Id, &t.Number, &t.Name); err != nil {
			return
		}
		teams = append(teams, t)
		return
	})
	if err != nil {
		return nil, err
	}
	// Refresh cache
	cache.m.Lock()
	defer cache.m.Unlock()
	cache.teams = teams
	for _, team := range teams {
		cache.teamById[team.Id] = team
	}

	tracef("Loaded %d teams.\n", len(teams))
	return teams, nil
}

func addTeam(db *sqlite.Conn, number int, name string) (id int64, err error) {
	tracef("Adding team (%d, %s)\n", number, name)
	s, err := db.Prepare(TEAM_INSERT)
	if err != nil {
		return
	}
	defer s.Finalize()
	id, err = s.Insert(number, name)
	if err != nil {
		return
	}
	tracef("Team (%d, %s) added\n", number, name)
	return
}

func updateTeam(db *sqlite.Conn, id int, number int, name string) (err error) {
	tracef("Updating team (%d, %d, %s)\n", id, number, name)
	s, err := db.Prepare(TEAM_UPDATE)
	if err != nil {
		return
	}
	defer s.Finalize()
	n, err := s.ExecDml(number, name, id)
	if err != nil {
		return
	} else if n != 1 {
		err = fmt.Errorf("No change while updating team (%d, %d, %s)\n", id, number, name)
		return
	}
	tracef("Team (%d, %d, %s) updated\n", id, number, name)
	return
}

func deleteTeam(db *sqlite.Conn, id int) (err error) {
	tracef("Deleting team (%d)\n", id)
	s, err := db.Prepare(TEAM_DELETE)
	if err != nil {
		return
	}
	defer s.Finalize()
	n, err := s.ExecDml(id)
	if err != nil {
		return
	} else if n != 1 {
		err = fmt.Errorf("No change while deleting team (%d)\n", id)
		return
	}
	tracef("Team (%d) deleted\n", id)
	return
}
