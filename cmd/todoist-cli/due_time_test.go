package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

// timedAPI serves one project and two tasks due today, one all-day and one at
// 15:00, in the shape API v1 returns (the time lives inside due.date).
func timedAPI(t *testing.T) http.Handler {
	t.Helper()
	today := time.Now().Format("2006-01-02")
	mux := http.NewServeMux()
	mux.HandleFunc("/labels", pageResponse([]todoist.Label{}))
	mux.HandleFunc("/sections", pageResponse([]todoist.Section{}))
	mux.HandleFunc("/projects", pageResponse([]todoist.Project{{ID: "p1", Name: "Work"}}))
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[
			{"id":"timed001","content":"Call back","project_id":"p1","priority":1,"labels":[],
			 "due":{"date":"` + today + `T15:00:00","string":"today 15:00","is_recurring":false}},
			{"id":"allday01","content":"Pay rent","project_id":"p1","priority":1,"labels":[],
			 "due":{"date":"` + today + `","string":"today","is_recurring":false}}
		]}`))
	})
	return mux
}

func TestSync_TimedDueDateIsListedToday(t *testing.T) {
	newTestEnv(t, timedAPI(t))
	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Call back") {
		t.Errorf("task due today at 15:00 missing from ls:\n%s", out)
	}
	if !strings.Contains(out, "today 15:00") {
		t.Errorf("expected the due time next to 'today':\n%s", out)
	}

	out, err = runCmd(t, "stats")
	if err != nil {
		t.Fatal(err)
	}
	assertStat(t, out, "due today", 2)
	assertStat(t, out, "due this week", 0)
}

func TestSync_TimedDueDateJSONSplitsDateAndDatetime(t *testing.T) {
	newTestEnv(t, timedAPI(t))
	if _, err := runCmd(t, "sync"); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, "ls", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got []jsonTask
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	today := time.Now().Format("2006-01-02")
	for _, task := range got {
		if task.Due == nil || task.Due.Date != today {
			t.Errorf("%s: due.date = %+v, want %q", task.ID, task.Due, today)
			continue
		}
		if task.ID == "timed001" && (task.Due.Datetime == nil || *task.Due.Datetime != today+"T15:00:00") {
			t.Errorf("timed task: due.datetime = %v, want the full value", task.Due.Datetime)
		}
		if task.ID == "allday01" && task.Due.Datetime != nil {
			t.Errorf("all-day task: due.datetime = %q, want null", *task.Due.Datetime)
		}
	}
}

func TestFormatDue_ShowsTimeWhenPresent(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	cases := []struct{ date, datetime, want string }{
		{today, "", "today"},
		{today, today + "T15:00:00", "today 15:00"},
		{tomorrow, tomorrow + "T09:30:00", "tomorrow 09:30"},
		{"2030-01-02", "2030-01-02T08:00:00", "2030-01-02 08:00"},
	}
	for _, c := range cases {
		if got := formatDue(c.date, c.datetime); got != c.want {
			t.Errorf("formatDue(%q, %q) = %q, want %q", c.date, c.datetime, got, c.want)
		}
	}
}

func TestFormatDue_UTCDatetimeIsShownInLocalTime(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	old := time.Local
	time.Local = loc
	defer func() { time.Local = old }()
	if got := formatDue("2030-01-02", "2030-01-02T14:00:00.000000Z"); got != "2030-01-02 15:00" {
		t.Errorf("got %q, want the Berlin local time 15:00", got)
	}
}
