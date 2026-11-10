package main

import (
	"testing"
	"time"
)

// completed_at is UTC; "done today" must compare it with the start of the
// local day, not with the local date as a string.
func TestCompletedSince_ComparesInstants(t *testing.T) {
	cases := []struct {
		zone, completedAt string
		want              bool
	}{
		// 19:00 the evening before in Honolulu (UTC-10): not today.
		{"Pacific/Honolulu", "2026-11-08T05:00:00.000000Z", false},
		// 01:00 local in Honolulu: today.
		{"Pacific/Honolulu", "2026-11-08T11:00:00.000000Z", true},
		// 02:00 local in Kiritimati (UTC+14), still the 7th in UTC: today.
		{"Pacific/Kiritimati", "2026-11-07T12:00:00.000000Z", true},
		// 23:00 the evening before in Kiritimati: not today.
		{"Pacific/Kiritimati", "2026-11-07T09:00:00Z", false},
	}
	for _, c := range cases {
		loc, err := time.LoadLocation(c.zone)
		if err != nil {
			t.Fatal(err)
		}
		startOfDay := time.Date(2026, 11, 8, 0, 0, 0, 0, loc)
		if got := completedSince(c.completedAt, startOfDay); got != c.want {
			t.Errorf("%s: completedSince(%s) = %v, want %v", c.zone, c.completedAt, got, c.want)
		}
	}
}
