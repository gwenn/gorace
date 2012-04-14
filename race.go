package main

import (
	"encoding/json"
	"errors"
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
	RACE_TMPL      = "race.html"
	TIME_LOGS_TMPL = "timelogs.html"
	RESULTS_TMPL   = "results.html"
)

func errorHandler(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func raceHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	teams, err := loadTeams(db)
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, RACE_TMPL, teams)
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

func fixTimeLog(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	return // TODO update or delete
}

func displayResults(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	results, err := loadResults(db)
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, RESULTS_TMPL, results)
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
	path.Join(TMPL_PATH, RACE_TMPL),
	path.Join(TMPL_PATH, TIME_LOGS_TMPL),
	path.Join(TMPL_PATH, RESULTS_TMPL)))

func main() {
	fileServer := http.FileServer(http.Dir(STATIC_PATH))
	http.Handle("/", fileServer)

	http.HandleFunc("/race", makeHandler(raceHandler))
	http.HandleFunc("/timelogs/add", makeHandler(addTimeLogs))
	http.HandleFunc("/timelogs/list", makeHandler(displayTimeLogs))
	http.HandleFunc("/timelogs/fix", makeHandler(fixTimeLog))
	http.HandleFunc("/results", makeHandler(displayResults))

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
