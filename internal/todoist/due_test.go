package todoist

import (
	"encoding/json"
	"testing"
	"time"
)

// API v1 carries a due time inside "date"; the client splits it so Date is
// always the local calendar day and Datetime the full value.
func TestDue_UnmarshalSplitsDateAndTime(t *testing.T) {
	t.Setenv("TZ", "UTC")
	cases := []struct {
		name, in, date, datetime string
	}{
		{"all day", `{"date":"2026-11-09","string":"9 Nov","is_recurring":false}`, "2026-11-09", ""},
		{"floating time", `{"date":"2026-11-09T15:00:00","string":"9 Nov 15:00","is_recurring":false}`, "2026-11-09", "2026-11-09T15:00:00"},
		{"fixed timezone", `{"date":"2026-11-09T15:00:00.000000Z","timezone":"Europe/Berlin","string":"9 Nov 16:00","is_recurring":true}`, "2026-11-09", "2026-11-09T15:00:00.000000Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d Due
			if err := json.Unmarshal([]byte(c.in), &d); err != nil {
				t.Fatal(err)
			}
			if d.Date != c.date || d.Datetime != c.datetime {
				t.Errorf("Date=%q Datetime=%q, want %q %q", d.Date, d.Datetime, c.date, c.datetime)
			}
		})
	}
}

// A UTC datetime falls on the local calendar day, not the UTC one.
func TestDue_UnmarshalUTCDatetimeUsesLocalDate(t *testing.T) {
	t.Setenv("TZ", "Pacific/Kiritimati") // UTC+14
	time.Local, _ = time.LoadLocation("Pacific/Kiritimati")
	defer func() { time.Local = time.UTC }()
	var d Due
	if err := json.Unmarshal([]byte(`{"date":"2026-11-09T22:00:00.000000Z","timezone":"Pacific/Kiritimati"}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Date != "2026-11-10" {
		t.Errorf("Date = %q, want the local day 2026-11-10", d.Date)
	}
}
