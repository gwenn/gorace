package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gwenn/gosqlite"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
)

const (
	DB_PATH        = "db/race.sqlite"
	STATIC_PATH    = "static"
	TMPL_PATH      = "tmpl"
	LAPS_TMPL      = "laps.html"
	TIME_LOGS_TMPL = "timelogs.html"
	RESULTS_TMPL   = "results.html"
	RACE_TMPL      = "race.html"
	TEAMS_TMPL     = "teams.html"
)

func errorHandler(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func lapsHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	teams, err := loadTeams(db)
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, LAPS_TMPL, teams)
	return
}

func addTimeLogs(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	err = r.ParseForm()
	if err != nil {
		return
	}
	time := r.Form["time"]
	if len(time) == 0 || len(time[0]) == 0 {
		err = errors.New("Missing 'time' value")
		return
	}
	err = checkTime(time[0])
	if err != nil {
		return
	}
	teams := r.Form["teams"]
	if len(teams) == 0 || len(teams[0]) == 0 {
		err = errors.New("Missing 'teams' value(s)")
		return
	}
	var teamIds []int = make([]int, len(teams))
	for i, teamId := range teams {
		teamIds[i], err = strconv.Atoi(teamId)
		if err != nil {
			warn("Invalid team id: %q (%s)\n", teamId, err)
			return
		}
	}
	err = saveTimeLogs(db, teamIds, time[0])
	if err != nil {
		return
	}
	timeLogs := make([]TimeLog, len(teamIds))
	err = withTeamCache(db, func(cache *TeamCache) error {
		for i, teamId := range teamIds {
			timeLogs[i] = TimeLog{Team: cache.get(teamId), Time: time[0]}
		}
		return nil
	})
	b, err := json.Marshal(timeLogs)
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
	return
}

func displayTimeLogs(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	teamValue := r.FormValue("team") // optional
	var timeLogs []TimeLog
	if len(teamValue) > 0 {
		var teamId int
		teamId, err = strconv.Atoi(teamValue)
		if err != nil {
			warn("Invalid team id: %q (%s)\n", teamValue, err)
			return
		}
		timeLogs, err = loadTimeLogsByTeam(db, teamId)
	} else {
		limitValue := r.FormValue("limit") // optional
		limit := -1
		if len(limitValue) > 0 {
			limit, err = strconv.Atoi(limitValue)
			if err != nil {
				warn("Invalid limit: %q (%s)\n", limitValue, err)
				return
			}
		}
		timeLogs, err = loadTimeLogs(db, limit)
	}
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, TIME_LOGS_TMPL, timeLogs)
	return
}

func updateTimeLogHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	teamId, err := parseTeamId(r, "team")
	if err != nil {
		return
	}
	oldTime, err := parseTime(r, "old_time")
	if err != nil {
		return
	}
	newTime, err := parseTime(r, "new_time")
	if err != nil {
		return
	}
	err = updateTimeLog(db, teamId, oldTime, newTime)
	return
}
func deleteTimeLogHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	teamId, err := parseTeamId(r, "team")
	if err != nil {
		return
	}
	time, err := parseTime(r, "time")
	if err != nil {
		return
	}
	err = deleteTimeLog(db, teamId, time)
	return
}

func displayResults(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	results, err := loadResults(db)
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, RESULTS_TMPL, results)
	return
}

func displayRace(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	race, err := loadRace(db)
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, RACE_TMPL, race)
	return
}

func setStartTime(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	time, err := parseTime(r, "start_time")
	if err != nil {
		return
	}
	err = saveRace(db, time)
	if err != nil {
		return
	}
	http.Redirect(w, r, "/laps", http.StatusFound)
	return
}

func displayTeams(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	teams, err := loadTeams(db)
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, TEAMS_TMPL, teams)
	return
}
func addTeamHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	number, name, err := parseTeamNumberAndName(r)
	if err != nil {
		return
	}
	id, err := addTeam(db, number, name)
	if err != nil {
		return
	}
	b, err := json.Marshal(id)
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
	return
}
func updateTeamHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	id, err := parseTeamId(r, "id")
	if err != nil {
		return
	}
	number, name, err := parseTeamNumberAndName(r)
	if err != nil {
		return
	}
	err = updateTeam(db, id, number, name)
	return
}
func deleteTeamHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	id, err := parseTeamId(r, "id")
	if err != nil {
		return
	}
	err = deleteTeam(db, id)
	return
}
func parseTeamNumberAndName(r *http.Request) (number int, name string, err error) {
	numberValue := r.FormValue("number")
	if len(numberValue) == 0 {
		err = errors.New("Missing 'number' value")
		return
	}
	number, err = strconv.Atoi(numberValue)
	if err != nil {
		warn("Invalid team number: %q (%s)\n", numberValue, err)
		return
	}
	name = r.FormValue("name")
	if len(name) == 0 {
		err = errors.New("Missing 'name' value")
		return
	}
	return
}
func parseTeamId(r *http.Request, key string) (id int, err error) {
	idValue := r.FormValue(key)
	if len(idValue) == 0 {
		err = fmt.Errorf("Missing %q value", key)
		return
	}
	id, err = strconv.Atoi(idValue)
	if err != nil {
		warn("Invalid team id: %q (%s)\n", idValue, err)
		return
	}
	return
}
func parseTime(r *http.Request, key string) (time string, err error) {
	time = r.FormValue(key)
	if len(time) == 0 {
		err = fmt.Errorf("Missing %q value", key)
		return
	}
	err = checkTime(time)
	return
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, *sqlite.Conn) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db, err := getConn()
		if err != nil {
			errorHandler(w, err)
			return
		}
		defer releaseConn(db)
		err = fn(w, r, db)
		if err != nil {
			errorHandler(w, err)
		}
	}
}

var templates = template.Must(template.ParseFiles(
	path.Join(TMPL_PATH, LAPS_TMPL),
	path.Join(TMPL_PATH, TIME_LOGS_TMPL),
	path.Join(TMPL_PATH, RESULTS_TMPL),
	path.Join(TMPL_PATH, RACE_TMPL),
	path.Join(TMPL_PATH, TEAMS_TMPL)))

func main() {
	fileServer := http.FileServer(http.Dir(STATIC_PATH))
	http.Handle("/", fileServer)

	http.HandleFunc("/laps", makeHandler(lapsHandler))
	http.HandleFunc("/timelogs/add", makeHandler(addTimeLogs))
	http.HandleFunc("/timelogs", makeHandler(displayTimeLogs))
	http.HandleFunc("/timelogs/update", makeHandler(updateTimeLogHandler))
	http.HandleFunc("/timelogs/delete", makeHandler(deleteTimeLogHandler))
	http.HandleFunc("/results", makeHandler(displayResults))
	// admin pages
	http.HandleFunc("/race", makeHandler(displayRace))
	http.HandleFunc("/race/start", makeHandler(setStartTime))
	http.HandleFunc("/teams", makeHandler(displayTeams))
	http.HandleFunc("/teams/add", makeHandler(addTeamHandler))
	http.HandleFunc("/teams/update", makeHandler(updateTeamHandler))
	http.HandleFunc("/teams/delete", makeHandler(deleteTeamHandler))

	var interrupted = make(chan os.Signal)
	go func() {
		<-interrupted
		closePool()
		trace("Bye bye")
		os.Exit(0)
	}()
	signal.Notify(interrupted, os.Interrupt)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fatal("error running race webserver: %v\n", err)
	}
}
