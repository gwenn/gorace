package main

import (
	"github.com/gwenn/gosqlite"
	"sync"
)

const TEAM_QUERY = `SELECT id, number, name FROM team ORDER BY number`

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
