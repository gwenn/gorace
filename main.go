package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gwenn/gosqlite"
	"html/template"
	"net/http"
	//	"net/http/httputil"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
)

const (
	DB_PATH            = "db/race.sqlite"
	STATIC_PATH        = "static"
	TMPL_PATH          = "tmpl"
	LAPS_TMPL          = "laps.html"
	TIME_LOG_EDIT_TMPL = "timelog_edit.html"
	TIME_LOGS_TMPL     = "timelogs.html"
	TIME_LOGS_URL      = "/timelogs/"
	RESULTS_TMPL       = "results.html"
	RACE_TMPL          = "race.html"
	TEAMS_TMPL         = "teams.html"
	TEAMS_URL          = "/teams/"
	NAV_BAR_TMPL       = "navbar.html"
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

func timeLogsHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) error {
	switch r.Method {
	case "GET":
		teamValue := r.FormValue("team") // optional
		var err error
		var timeLogs []TimeLog
		if len(teamValue) > 0 {
			var teamId int
			teamId, err = strconv.Atoi(teamValue)
			if err != nil {
				warn("Invalid team id: %q (%s)\n", teamValue, err)
				return err
			}
			timeLogs, err = loadTimeLogsByTeam(db, teamId)
		} else {
			limitValue := r.FormValue("limit") // optional
			limit := -1
			if len(limitValue) > 0 {
				limit, err = strconv.Atoi(limitValue)
				if err != nil {
					warn("Invalid limit: %q (%s)\n", limitValue, err)
					return err
				}
			}
			timeLogs, err = loadTimeLogs(db, limit)
		}
		if err != nil {
			return err
		}
		return templates.ExecuteTemplate(w, TIME_LOGS_TMPL, timeLogs)
	case "POST":
		err := r.ParseForm()
		if err != nil {
			return err
		}
		time := r.Form["time"]
		if len(time) == 0 || len(time[0]) == 0 {
			return errors.New("Missing 'time' value")
		}
		err = checkTime(time[0])
		if err != nil {
			return err
		}
		teams := r.Form["teams"]
		if len(teams) == 0 || len(teams[0]) == 0 {
			return errors.New("Missing 'teams' value(s)")
		}
		var teamIds []int = make([]int, len(teams))
		for i, teamId := range teams {
			teamIds[i], err = strconv.Atoi(teamId)
			if err != nil {
				warn("Invalid team id: %q (%s)\n", teamId, err)
				return err
			}
		}
		err = saveTimeLogs(db, teamIds, time[0])
		if err != nil {
			return err
		}
		timeLogs := make([]TimeLog, len(teamIds))
		err = withTeamCache(db, func(cache *TeamCache) error {
			for i, teamId := range teamIds {
				timeLogs[i] = TimeLog{Team: cache.get(teamId), Time: time[0]}
			}
			return nil
		})
		if err != nil {
			return err
		}
		b, err := json.Marshal(timeLogs)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, err = w.Write(b)
		return err
	case "PUT":
		teamId, err := parseTeamId(r.FormValue("team"))
		if err != nil {
			return err
		}
		oldTime, err := parseTime(r.FormValue("old_time"))
		if err != nil {
			return err
		}
		newTime, err := parseTime(r.FormValue("new_time"))
		if err != nil {
			return err
		}
		return updateTimeLog(db, teamId, oldTime, newTime)
	case "DELETE":
		params := strings.Split(r.URL.Path[len(TIME_LOGS_URL):], "/")
		fmt.Printf("%#v\n", params)
		if len(params) < 2 {
			return errors.New("Missing 'team'/'time' value")
		}
		teamId, err := parseTeamId(params[0])
		if err != nil {
			return err
		}
		time, err := parseTime(params[1])
		if err != nil {
			return err
		}
		return deleteTimeLog(db, teamId, time)
	default:
		http.Error(w, "501 Not Implemented", http.StatusNotImplemented)
	}
	return nil
}

func resultsHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) (err error) {
	results, err := loadResults(db)
	if err != nil {
		return
	}
	err = templates.ExecuteTemplate(w, RESULTS_TMPL, results)
	return
}

func raceHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) error {
	switch r.Method {
	case "GET":
		race, err := loadRace(db)
		if err != nil {
			return err
		}
		return templates.ExecuteTemplate(w, RACE_TMPL, race)
	case "PUT":
		time, err := parseTime(r.FormValue("start_time"))
		if err != nil {
			return err
		}
		return saveRace(db, time)
	default:
		http.Error(w, "501 Not Implemented", http.StatusNotImplemented)
	}
	return nil
}

func teamsHandler(w http.ResponseWriter, r *http.Request, db *sqlite.Conn) error {
	switch r.Method {
	case "GET":
		teams, err := loadTeams(db)
		if err != nil {
			return err
		}
		return templates.ExecuteTemplate(w, TEAMS_TMPL, teams)
	case "POST":
		number, name, err := parseTeamNumberAndName(r)
		if err != nil {
			return err
		}
		id, err := addTeam(db, number, name)
		if err != nil {
			return err
		}
		b, err := json.Marshal(id)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, err = w.Write(b)
		return err
	case "PUT":
		id, err := parseTeamId(r.FormValue("tid"))
		if err != nil {
			return err
		}
		number, name, err := parseTeamNumberAndName(r)
		if err != nil {
			return err
		}
		return updateTeam(db, id, number, name)
	case "DELETE":
		id, err := parseTeamId(r.URL.Path[len(TEAMS_URL):])
		if err != nil {
			return err
		}
		return deleteTeam(db, id)
	default:
		http.Error(w, "501 Not Implemented", http.StatusNotImplemented)
	}
	return nil
}

func parseTeamNumberAndName(r *http.Request) (number int, name string, err error) {
	numberValue := r.FormValue("tnumber")
	if len(numberValue) == 0 {
		err = errors.New("Missing 'number' value")
		return
	}
	number, err = strconv.Atoi(numberValue)
	if err != nil {
		warn("Invalid team number: %q (%s)\n", numberValue, err)
		return
	}
	name = r.FormValue("tname")
	if len(name) == 0 {
		err = errors.New("Missing 'name' value")
		return
	}
	return
}
func parseTeamId(idValue string) (id int, err error) {
	if len(idValue) == 0 {
		err = fmt.Errorf("Missing team id")
		return
	}
	id, err = strconv.Atoi(idValue)
	if err != nil {
		warn("Invalid team id: %q (%s)\n", idValue, err)
		return
	}
	return
}
func parseTime(time string) (string, error) {
	if len(time) == 0 {
		return time, fmt.Errorf("Missing time value")
	}
	return time, checkTime(time)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, *sqlite.Conn) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		/*
			dump, err := httputil.DumpRequest(r, true)
			if err != nil {
				errorHandler(w, err)
			}
			warn("Dump: %s\n", string(dump))
		*/
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
	path.Join(TMPL_PATH, TIME_LOG_EDIT_TMPL),
	path.Join(TMPL_PATH, TIME_LOGS_TMPL),
	path.Join(TMPL_PATH, RESULTS_TMPL),
	path.Join(TMPL_PATH, RACE_TMPL),
	path.Join(TMPL_PATH, TEAMS_TMPL),
	path.Join(TMPL_PATH, NAV_BAR_TMPL)))

func main() {
	fileServer := http.FileServer(http.Dir(STATIC_PATH))
	http.Handle("/", fileServer)

	http.HandleFunc("/laps", makeHandler(lapsHandler))
	http.HandleFunc(TIME_LOGS_URL, makeHandler(timeLogsHandler))
	http.HandleFunc("/results", makeHandler(resultsHandler))
	// admin pages
	http.HandleFunc("/race/", makeHandler(raceHandler))
	http.HandleFunc(TEAMS_URL, makeHandler(teamsHandler))

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
