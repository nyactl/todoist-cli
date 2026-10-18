package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseSince_NamedPeriodsStartAtMidnight(t *testing.T) {
	now := time.Now()
	midnight := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	cases := map[string]time.Time{
		"":      midnight(now),
		"today": midnight(now),
		"week":  midnight(now.AddDate(0, 0, -7)),
		"month": midnight(now.AddDate(0, -1, 0)),
		"year":  midnight(now.AddDate(-1, 0, 0)),
	}
	for in, want := range cases {
		got, err := parseSince(in)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseSince(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}

func TestParseSince_RelativePeriodsStartAtMidnight(t *testing.T) {
	now := time.Now()
	midnight := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	cases := map[string]time.Time{
		"1d":  midnight(now.AddDate(0, 0, -1)),
		"3d":  midnight(now.AddDate(0, 0, -3)),
		"2w":  midnight(now.AddDate(0, 0, -14)),
		"6m":  midnight(now.AddDate(0, -6, 0)),
		"30d": midnight(now.AddDate(0, 0, -30)),
	}
	for in, want := range cases {
		got, err := parseSince(in)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseSince(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}

func TestParseSince_SevenDaysEqualsWeek(t *testing.T) {
	week, _ := parseSince("week")
	d7, _ := parseSince("7d")
	w1, _ := parseSince("1w")
	if !d7.Equal(week) || !w1.Equal(week) {
		t.Errorf("week = %v, 7d = %v, 1w = %v; want all equal", week, d7, w1)
	}
}

func TestParseSince_Invalid(t *testing.T) {
	cases := map[string]string{
		"x":    "unknown period",
		"0d":   "unknown period",
		"-3d":  "unknown period",
		"d":    "unknown period",
		"abcd": "unknown period",
		"3y":   "unknown unit",
		"1.5d": "unknown period",
		"3dd":  "unknown period",
		"2 d":  "unknown period",
		"+3d":  "unknown period",
	}
	for in, want := range cases {
		if _, err := parseSince(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseSince(%q) error = %v, want %q", in, err, want)
		}
	}
}

func TestLsDone_InvalidPeriodErrors(t *testing.T) {
	newTestEnv(t, completedHandler(""))
	if _, err := runCmd(t, "ls", "--done", "3y"); err == nil || !strings.Contains(err.Error(), "unknown unit") {
		t.Errorf("err = %v", err)
	}
}

func TestFormatCompletedAt(t *testing.T) {
	today := time.Now()
	todayISO := time.Date(today.Year(), today.Month(), today.Day(), 9, 30, 0, 0, time.Local).UTC().Format(time.RFC3339)
	if got := formatCompletedAt(todayISO); got != "09:30" {
		t.Errorf("today = %q, want 09:30", got)
	}
	old := time.Date(2026, 3, 4, 8, 5, 0, 0, time.Local).UTC()
	if got := formatCompletedAt(old.Format("2006-01-02T15:04:05.000000Z")); got != "03-04 08:05" {
		t.Errorf("microsecond format = %q, want 03-04 08:05", got)
	}
	if got := formatCompletedAt("not a time"); got != "not a time" {
		t.Errorf("unparseable = %q, want input unchanged", got)
	}
}

func TestPriorityMark(t *testing.T) {
	want := map[int]string{4: "!!", 3: "! ", 2: "· ", 1: "  ", 0: "  "}
	for p, w := range want {
		if got := priorityMark(p); got != w {
			t.Errorf("priorityMark(%d) = %q, want %q", p, got, w)
		}
	}
}
