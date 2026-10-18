package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:               "show <id>",
	Short:             "Show full task details (live API call; cached details when offline)",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: taskCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// resolve prefix → full ID from local cache if possible
		taskID := args[0]
		var cached *todoist.Task
		if conn, err := db.Open(); err == nil {
			if t, err := tasks.ByID(ctx, conn, args[0]); err == nil {
				taskID = t.ID
				cached = cachedTask(ctx, conn, t)
			}
			conn.Close()
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		t, err := client.GetTask(ctx, taskID)
		offline := false
		if err != nil {
			if cached == nil || !isNetworkError(err) {
				return err
			}
			t, offline = cached, true
			fmt.Fprintln(os.Stderr, "offline — showing cached details; comments are not available")
		}

		// Comments + resolved author names — shared by both output modes.
		var comments []todoist.Comment
		if !offline {
			comments, _ = client.GetComments(ctx, t.ID)
		}
		authorProject := ""
		if len(comments) > 0 {
			authorProject = t.ProjectID
		}
		authorName := commentAuthors(ctx, client, authorProject)
		projectName := projectNameByID(ctx, t.ProjectID)

		var subs []tasks.Task
		if conn, err := db.Open(); err == nil {
			subs, _ = tasks.Subtasks(ctx, conn, t.ID)
			conn.Close()
		}

		if showJSON {
			return emitTaskJSON(t, projectName, sectionNameByID(ctx, t.SectionID), subs, comments, authorName)
		}

		fmt.Printf("%s  %s\n", shortID(ctx, t.ID), t.Content)
		if t.Description != "" {
			fmt.Printf("\n%s\n", t.Description)
		}

		// Metadata block — aligned, only fields that carry information.
		var meta []string
		if projectName != "" && projectName != "Inbox" {
			meta = append(meta, fmt.Sprintf("%-9s %s", "project", projectName))
		}
		if t.Priority > todoist.PriorityNormal {
			meta = append(meta, fmt.Sprintf("%-9s %d (%s)", "priority", t.Priority, priorityWord(t.Priority)))
		}
		if t.Due != nil {
			due := t.Due.Date
			if t.Due.Datetime != "" {
				due = t.Due.Datetime
			}
			line := fmt.Sprintf("%-9s %s", "due", due)
			if t.Due.String != "" {
				line += fmt.Sprintf("  (%s)", t.Due.String)
			}
			meta = append(meta, line)
		}
		if t.Deadline != nil && t.Deadline.Date != "" {
			meta = append(meta, fmt.Sprintf("%-9s %s", "deadline", t.Deadline.Date))
		}
		if len(t.Labels) > 0 {
			meta = append(meta, fmt.Sprintf("%-9s %s", "labels", strings.Join(t.Labels, ", ")))
		}
		if len(meta) > 0 {
			fmt.Println()
			for _, line := range meta {
				fmt.Println(line)
			}
		}

		if len(subs) > 0 {
			ids := make([]string, len(subs))
			for i, s := range subs {
				ids[i] = s.ID
			}
			short := shortIDs(ctx, ids...)
			width := 0
			for _, id := range short {
				width = max(width, len(id))
			}
			fmt.Println("\nsubtasks")
			for _, s := range subs {
				fmt.Printf("  %-*s  %s\n", width, short[s.ID], s.Content)
			}
		}

		if len(comments) > 0 {
			fmt.Println("\ncomments")
			for i, c := range comments {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("  %s  %s\n", formatCommentTime(c.PostedAt), authorName(c.PostedUID))
				fmt.Printf("  %s\n", c.Content)
			}
		}

		if t.URL != "" {
			fmt.Printf("\n%s\n", t.URL)
		}
		return nil
	},
}

// projectNameByID resolves a project ID to its name from the local cache.
// Returns "" when unknown (e.g. cache miss), so callers simply omit the line.
func projectNameByID(ctx context.Context, id string) string {
	if id == "" {
		return ""
	}
	conn, err := db.Open()
	if err != nil {
		return ""
	}
	defer conn.Close()
	var name string
	conn.QueryRowContext(ctx, `SELECT name FROM projects WHERE id = ?`, id).Scan(&name)
	return name
}

// formatCommentTime renders a comment's posted_at (UTC ISO) as a full local
// date-time. Comments can be old, so the year is always shown (unlike the
// relative format used for the completed-task view).
func formatCommentTime(iso string) string {
	t, err := time.Parse("2006-01-02T15:04:05.000000Z", iso)
	if err != nil {
		t, err = time.Parse(time.RFC3339, iso)
		if err != nil {
			if len(iso) >= 10 {
				return iso[:10]
			}
			return iso
		}
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// sectionNameByID resolves a section ID to its name from the local cache.
func sectionNameByID(ctx context.Context, id string) string {
	if id == "" {
		return ""
	}
	conn, err := db.Open()
	if err != nil {
		return ""
	}
	defer conn.Close()
	var name string
	conn.QueryRowContext(ctx, `SELECT name FROM sections WHERE id = ?`, id).Scan(&name)
	return name
}

// emitTaskJSON prints the task as a machine-readable JSON object matching the
// documented schema (issue #19). Absent fields serialize as null; labels and
// comments are always arrays.
func emitTaskJSON(t *todoist.Task, project, section string, subs []tasks.Task, comments []todoist.Comment, authorName func(string) string) error {
	type jsonSubtask struct {
		ID       string   `json:"id"`
		Content  string   `json:"content"`
		Priority int      `json:"priority"`
		Due      *jsonDue `json:"due"`
	}
	type showComment struct {
		ID       string `json:"id"`
		PostedAt string `json:"posted_at"`
		Author   string `json:"author"`
		Content  string `json:"content"`
	}
	out := struct {
		ID          string        `json:"id"`
		Content     string        `json:"content"`
		Description string        `json:"description"`
		Project     *string       `json:"project"`
		Section     *string       `json:"section"`
		Priority    int           `json:"priority"`
		Labels      []string      `json:"labels"`
		Due         *jsonDue      `json:"due"`
		Deadline    *string       `json:"deadline"`
		CreatedAt   string        `json:"created_at"`
		URL         string        `json:"url"`
		Subtasks    []jsonSubtask `json:"subtasks"`
		Comments    []showComment `json:"comments"`
	}{
		ID:          t.ID,
		Content:     t.Content,
		Description: t.Description,
		Project:     nilIfEmpty(project),
		Section:     nilIfEmpty(section),
		Priority:    t.Priority,
		Labels:      []string{},
		CreatedAt:   t.CreatedAt,
		URL:         t.URL,
		Subtasks:    []jsonSubtask{},
		Comments:    []showComment{},
	}
	if len(t.Labels) > 0 {
		out.Labels = t.Labels
	}
	if t.Due != nil {
		out.Due = &jsonDue{Date: t.Due.Date, Datetime: nilIfEmpty(t.Due.Datetime), Recurring: t.Due.IsRecurring, String: nilIfEmpty(t.Due.String)}
	}
	if t.Deadline != nil {
		out.Deadline = nilIfEmpty(t.Deadline.Date)
	}
	for _, s := range subs {
		out.Subtasks = append(out.Subtasks, jsonSubtask{ID: s.ID, Content: s.Content, Priority: s.Priority, Due: cachedDue(s)})
	}
	for _, c := range comments {
		out.Comments = append(out.Comments, showComment{
			ID: c.ID, PostedAt: c.PostedAt, Author: authorName(c.PostedUID), Content: c.Content,
		})
	}

	return emitJSON(out)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// priorityWord maps a Todoist priority (4 highest) to a human label.
func priorityWord(p int) string {
	switch p {
	case todoist.PriorityUrgent:
		return "urgent"
	case todoist.PriorityHigh:
		return "high"
	case todoist.PriorityMedium:
		return "medium"
	default:
		return "normal"
	}
}

var (
	showProject string
	showJSON    bool
)

func init() {
	showCmd.Flags().StringVarP(&showProject, "project", "p", "", "filter task completion by project name")
	showCmd.Flags().BoolVar(&showJSON, "json", false, "emit the task as a machine-readable JSON object")
	showCmd.RegisterFlagCompletionFunc("project", projectCompleter)
	root.AddCommand(showCmd)
}

// isNetworkError reports whether err means the API could not be reached at
// all, as opposed to an error response from it (which must not be masked by
// stale cache, e.g. a 404 for a task deleted elsewhere).
func isNetworkError(err error) bool {
	var apiErr *todoist.APIError
	if errors.As(err, &apiErr) {
		return false
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// cachedTask rebuilds the API view of a task from the local cache.
func cachedTask(ctx context.Context, conn *sql.DB, c *tasks.Task) *todoist.Task {
	t := &todoist.Task{
		ID:          c.ID,
		Content:     c.Content,
		Description: c.Description,
		Priority:    c.Priority,
		ProjectID:   c.ProjectID,
		SectionID:   c.SectionID,
		ParentID:    c.ParentID,
		URL:         c.URL,
	}
	t.Labels, _ = tasks.Labels(ctx, conn, c.ID)
	if c.DueDate != "" {
		t.Due = &todoist.Due{Date: c.DueDate, Datetime: c.DueDatetime, String: c.DueString, IsRecurring: c.DueRecurring}
	}
	if c.DeadlineDate != "" {
		t.Deadline = &todoist.Deadline{Date: c.DeadlineDate}
	}
	conn.QueryRowContext(ctx, `SELECT created_at FROM tasks WHERE id = ?`, c.ID).Scan(&t.CreatedAt)
	return t
}
