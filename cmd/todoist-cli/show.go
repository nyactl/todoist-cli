package main

import (
	"context"
	"fmt"
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
	Short:             "Show full task details (live API call)",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: taskCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// resolve prefix → full ID from local cache if possible
		taskID := args[0]
		if conn, err := db.Open(); err == nil {
			if t, err := tasks.ByID(ctx, conn, args[0]); err == nil {
				taskID = t.ID
			}
			conn.Close()
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		t, err := client.GetTask(ctx, taskID)
		if err != nil {
			return err
		}

		fmt.Printf("%s  %s\n", shortID(t.ID), t.Content)
		if t.Description != "" {
			fmt.Printf("\n%s\n", t.Description)
		}

		// Metadata block — aligned, only fields that carry information.
		var meta []string
		if pn := projectNameByID(ctx, t.ProjectID); pn != "" && pn != "Inbox" {
			meta = append(meta, fmt.Sprintf("%-9s %s", "project", pn))
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

		// subtasks from local cache
		if conn, err := db.Open(); err == nil {
			if subs, err := tasks.Subtasks(ctx, conn, t.ID); err == nil && len(subs) > 0 {
				fmt.Println("\nsubtasks")
				for _, s := range subs {
					fmt.Printf("  %s  %s\n", shortID(s.ID), s.Content)
				}
			}
			conn.Close()
		}

		// comments from API, with author names resolved from the project's
		// collaborators (falls back to the raw uid if resolution is unavailable)
		comments, err := client.GetComments(ctx, t.ID)
		if err == nil && len(comments) > 0 {
			authors := map[string]string{}
			if collabs, err := client.GetCollaborators(ctx, t.ProjectID); err == nil {
				for _, c := range collabs {
					authors[c.ID] = c.Name
				}
			}
			fmt.Println("\ncomments")
			for i, c := range comments {
				if i > 0 {
					fmt.Println()
				}
				author := authors[c.PostedUID]
				if author == "" {
					author = c.PostedUID
				}
				fmt.Printf("  %s  %s\n", formatCommentTime(c.PostedAt), author)
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

var showProject string

func init() {
	showCmd.Flags().StringVarP(&showProject, "project", "p", "", "filter task completion by project name")
	showCmd.RegisterFlagCompletionFunc("project", projectCompleter)
	root.AddCommand(showCmd)
}
