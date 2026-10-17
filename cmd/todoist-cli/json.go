package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/nyactl/todoist-cli/internal/tasks"
	"github.com/nyactl/todoist-cli/internal/todoist"
)

// jsonOut is the --json flag shared by every list command. Task objects use the
// same field names as `show --json` so one parser handles all commands.
var jsonOut bool

type jsonDue struct {
	Date      string  `json:"date"`
	Datetime  *string `json:"datetime"`
	Recurring bool    `json:"recurring"`
	String    *string `json:"string"`
}

type jsonTask struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Project     *string  `json:"project"`
	Section     *string  `json:"section"`
	ParentID    *string  `json:"parent_id"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	Due         *jsonDue `json:"due"`
	Deadline    *string  `json:"deadline"`
	URL         string   `json:"url"`
}

type jsonComment struct {
	ID       string `json:"id"`
	TaskID   string `json:"task_id"`
	PostedAt string `json:"posted_at"`
	Author   string `json:"author"`
	Content  string `json:"content"`
}

func cachedDue(t tasks.Task) *jsonDue {
	if t.DueDate == "" {
		return nil
	}
	return &jsonDue{Date: t.DueDate, Datetime: nilIfEmpty(t.DueDatetime), Recurring: t.DueRecurring, String: nilIfEmpty(t.DueString)}
}

func tasksJSON(ctx context.Context, conn *sql.DB, ts []tasks.Task) ([]jsonTask, error) {
	out := make([]jsonTask, 0, len(ts))
	for _, t := range ts {
		labels, err := tasks.Labels(ctx, conn, t.ID)
		if err != nil {
			return nil, err
		}
		if labels == nil {
			labels = []string{}
		}
		out = append(out, jsonTask{
			ID:          t.ID,
			Content:     t.Content,
			Description: t.Description,
			Project:     nilIfEmpty(t.ProjectName),
			Section:     nilIfEmpty(t.SectionName),
			ParentID:    nilIfEmpty(t.ParentID),
			Priority:    t.Priority,
			Labels:      labels,
			Due:         cachedDue(t),
			Deadline:    nilIfEmpty(t.DeadlineDate),
			URL:         t.URL,
		})
	}
	return out, nil
}

func emitTasksJSON(ctx context.Context, conn *sql.DB, ts []tasks.Task) error {
	out, err := tasksJSON(ctx, conn, ts)
	if err != nil {
		return err
	}
	return emitJSON(out)
}

func emitJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

// commentAuthors maps user IDs to display names for a project's comments.
// Unresolvable IDs (no access, API error) fall back to the raw ID.
func commentAuthors(ctx context.Context, client *todoist.Client, projectID string) func(string) string {
	names := map[string]string{}
	if projectID != "" {
		if collabs, err := client.GetCollaborators(ctx, projectID); err == nil {
			for _, c := range collabs {
				names[c.ID] = c.Name
			}
		}
	}
	return func(uid string) string {
		if n := names[uid]; n != "" {
			return n
		}
		return uid
	}
}
