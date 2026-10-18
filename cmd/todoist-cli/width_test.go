package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

func TestTruncate_DisplayWidth(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"Gemischte Tüte kaufen", 15, "Gemischte Tüte…"},
		{"日本語の勉強をする", 10, "日本語の…"},
		{"日本語の勉強をする", 9, "日本語の…"},
		{"line\nbreak", 20, "line break"},
	}
	for _, c := range cases {
		got := truncate(c.in, c.n)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("truncate(%q, %d) produced invalid UTF-8: %q", c.in, c.n, got)
		}
		if w := runewidth.StringWidth(got); w > c.n {
			t.Errorf("truncate(%q, %d) is %d columns wide", c.in, c.n, w)
		}
	}
}

func TestPadRight_DisplayWidth(t *testing.T) {
	for _, s := range []string{"abc", "Tüte", "日本語", "· x"} {
		if w := runewidth.StringWidth(padRight(s, 12)); w != 12 {
			t.Errorf("padRight(%q, 12) is %d columns wide", s, w)
		}
	}
}

func TestTerminalWidth_ColumnsOverride(t *testing.T) {
	t.Setenv("COLUMNS", "77")
	if got := terminalWidth(); got != 77 {
		t.Errorf("terminalWidth() = %d, want 77", got)
	}
}

// columnsOf returns the display column of each occurrence of sub in line.
func columnsOf(line, sub string) []int {
	var cols []int
	for i := 0; ; {
		j := strings.Index(line[i:], sub)
		if j < 0 {
			return cols
		}
		cols = append(cols, runewidth.StringWidth(line[:i+j]))
		i += j + len(sub)
	}
}

func TestLs_AlignsNonASCIIContent(t *testing.T) {
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Home")
	for _, c := range []string{"Buy milk", "Gemischte Tüte kaufen", "日本語の勉強をする"} {
		env.conn.Exec(`INSERT INTO tasks (id, content, project_id, due_date) VALUES (?, ?, 'p1', date('now', 'localtime'))`, "id-"+c, c)
	}

	out, err := runCmd(t, "ls")
	if err != nil {
		t.Fatal(err)
	}
	var cols []int
	for _, line := range strings.Split(out, "\n") {
		if c := columnsOf(line, "today"); len(c) == 1 {
			cols = append(cols, c[0])
		}
	}
	if len(cols) != 3 || cols[0] != cols[1] || cols[1] != cols[2] {
		t.Errorf("due column not aligned (columns %v):\n%s", cols, out)
	}
}

func TestLs_BoardAlignsAndFitsWidth(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	env := newTestEnv(t, nil)
	hSeedProject(t, env.conn, "p1", "Home")
	hSeedSection(t, env.conn, "s1", "Einkäufe", "p1", 1)
	hSeedSection(t, env.conn, "s2", "日本語", "p1", 2)
	hSeedSection(t, env.conn, "s3", "Done", "p1", 3)
	hSeedTask(t, env.conn, "t1", "Gemischte Tüte kaufen und abholen", "p1", "s1")
	hSeedTask(t, env.conn, "t2", "日本語の勉強をする毎日", "p1", "s2")
	hSeedTask(t, env.conn, "t3", "Plain ASCII task", "p1", "s3")
	hSeedTask(t, env.conn, "t4", "Second ASCII task", "p1", "s1")
	if _, err := runCmd(t, "cd", "Home"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "ls", "-b")
	if err != nil {
		t.Fatal(err)
	}
	var want []int
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := runewidth.StringWidth(line); w > 80 {
			t.Errorf("line is %d columns, terminal is 80: %q", w, line)
		}
		cols := columnsOf(line, "│")
		if len(cols) == 0 {
			continue
		}
		if want == nil {
			want = cols
		} else if !slicesEqual(cols, want) {
			t.Errorf("separators at %v, header at %v: %q", cols, want, line)
		}
	}
}

func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
