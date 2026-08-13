package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/db"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var (
	labelsRmForce  bool
	labelsRmUnused bool
)

var labelsCmd = &cobra.Command{
	Use:               "labels",
	Short:             "List all labels (id, name, kind — tab-separated; kind is personal or shared)",
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()
		return printLabels(cmd.Context(), conn)
	},
}

// A label name may be a "personal" label (in the curated list, has an ID,
// managed via /labels/{id}) or a "shared" label (only a name on tasks, managed
// via /labels/shared/*). The CLI's own add -l / edit --add-label create shared
// labels, so both kinds must be handled.
type labelTarget struct {
	name     string
	id       string // personal label ID; empty for a shared label
	personal bool
	count    int // tasks in the local cache carrying this label
}

// resolveLabel classifies a name against the local cache: whether it is a
// personal label (and its ID) and how many cached tasks carry it.
func resolveLabel(ctx context.Context, conn *sql.DB, name string) (labelTarget, error) {
	t := labelTarget{name: name}
	err := conn.QueryRowContext(ctx, `SELECT id FROM labels WHERE name = ?`, name).Scan(&t.id)
	switch {
	case err == nil:
		t.personal = true
	case err == sql.ErrNoRows:
		// not a personal label — may still be a shared label on tasks
	default:
		return t, err
	}
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM task_labels WHERE label_name = ?`, name).Scan(&t.count); err != nil {
		return t, err
	}
	return t, nil
}

var labelsRmCmd = &cobra.Command{
	Use:               "rm <name>...",
	Short:             "Delete labels (detaches them from every task)",
	ValidArgsFunction: labelCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if labelsRmUnused && len(args) > 0 {
			return fmt.Errorf("--unused cannot be combined with label names")
		}
		if !labelsRmUnused && len(args) == 0 {
			return fmt.Errorf("provide at least one label name, or use --unused")
		}

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		targets, err := labelDeleteTargets(ctx, conn, args)
		if err != nil {
			return err
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		out := cmd.OutOrStdout()
		scanner := bufio.NewScanner(cmd.InOrStdin())
		deleted := 0
		for _, t := range targets {
			// Deleting a label detaches it from every task, so confirm when it is
			// in use — unless forced (scripted cleanup) or empty (the safe case).
			if t.count > 0 && !labelsRmForce {
				fmt.Fprintf(out, "label %q is on %d task(s) — delete? [y/N] ", t.name, t.count)
				if !scanner.Scan() {
					break
				}
				ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if ans != "y" && ans != "yes" {
					fmt.Fprintf(out, "skipped: %s\n", t.name)
					continue
				}
			}
			if t.personal {
				err = client.DeleteLabel(ctx, t.id)
			} else {
				err = client.RemoveSharedLabel(ctx, t.name)
			}
			if err != nil {
				return fmt.Errorf("delete %q: %w", t.name, err)
			}
			conn.ExecContext(ctx, `DELETE FROM labels WHERE name = ?`, t.name)
			conn.ExecContext(ctx, `DELETE FROM task_labels WHERE label_name = ?`, t.name)
			deleted++
			if !labelsRmUnused {
				fmt.Fprintf(out, "deleted: %s\n", t.name)
			}
		}
		if labelsRmUnused {
			fmt.Fprintf(out, "removed %d unused label(s)\n", deleted)
		}
		return nil
	},
}

// labelDeleteTargets resolves the labels to delete — either the named ones (with
// their cache usage count) or, with --unused, every label carrying no cached task.
func labelDeleteTargets(ctx context.Context, conn *sql.DB, names []string) ([]labelTarget, error) {
	if labelsRmUnused {
		// Unused = personal labels carrying no cached task. Shared labels only
		// exist while attached to a task, so there is nothing unused to prune there.
		rows, err := conn.QueryContext(ctx, `
			SELECT l.id, l.name FROM labels l
			WHERE NOT EXISTS (SELECT 1 FROM task_labels tl WHERE tl.label_name = l.name)
			ORDER BY l.ord`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var targets []labelTarget
		for rows.Next() {
			t := labelTarget{personal: true}
			if err := rows.Scan(&t.id, &t.name); err != nil {
				return nil, err
			}
			targets = append(targets, t)
		}
		return targets, rows.Err()
	}

	targets := make([]labelTarget, 0, len(names))
	for _, name := range names {
		t, err := resolveLabel(ctx, conn, name)
		if err != nil {
			return nil, err
		}
		if !t.personal && t.count == 0 {
			return nil, fmt.Errorf("label %q not found in local cache — run: todoist-cli sync", name)
		}
		targets = append(targets, t)
	}
	return targets, nil
}

var labelsRenameCmd = &cobra.Command{
	Use:               "rename <old> <new>",
	Aliases:           []string{"mv"}, // consistent with `projects mv`
	Short:             "Rename a label (updates every task carrying it)",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: labelCompleter,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		oldName, newName := args[0], args[1]
		if oldName == newName {
			return fmt.Errorf("old and new names are the same")
		}

		conn, err := db.Open()
		if err != nil {
			return err
		}
		defer conn.Close()

		src, err := resolveLabel(ctx, conn, oldName)
		if err != nil {
			return err
		}
		if !src.personal && src.count == 0 {
			return fmt.Errorf("label %q not found in local cache — run: todoist-cli sync", oldName)
		}
		// Collision: the new name must not already exist as a personal label or on tasks.
		dst, err := resolveLabel(ctx, conn, newName)
		if err != nil {
			return err
		}
		if dst.personal || dst.count > 0 {
			return fmt.Errorf("label %q already exists", newName)
		}

		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)

		if src.personal {
			_, err = client.UpdateLabel(ctx, src.id, todoist.UpdateLabelRequest{Name: newName})
		} else {
			err = client.RenameSharedLabel(ctx, oldName, newName)
		}
		if err != nil {
			return err
		}
		// Todoist renames the label on every task server-side; mirror that locally.
		conn.ExecContext(ctx, `UPDATE labels SET name = ? WHERE name = ?`, newName, oldName)
		// UPDATE OR IGNORE avoids the (task_id, label_name) PK collision if a task
		// somehow already carries both names; the leftover old rows are then dropped.
		conn.ExecContext(ctx, `UPDATE OR IGNORE task_labels SET label_name = ? WHERE label_name = ?`, newName, oldName)
		conn.ExecContext(ctx, `DELETE FROM task_labels WHERE label_name = ?`, oldName)

		fmt.Fprintf(cmd.OutOrStdout(), "renamed: %s -> %s\n", oldName, newName)
		return nil
	},
}

func printLabels(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, allLabelsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, kind string
		if err := rows.Scan(&id, &name, &kind); err != nil {
			return err
		}
		// id\tname\tkind — id is empty for shared labels (they have no ID).
		fmt.Printf("%s\t%s\t%s\n", id, name, kind)
	}
	return rows.Err()
}

// allLabelsQuery lists every label the user has — personal labels (from the
// curated list) and shared labels (names that live only on tasks) — so that
// `labels`, tab-completion, rm and rename all share one view. Personal labels
// come first in their order, then shared labels alphabetically.
const allLabelsQuery = `
	SELECT id, name, kind FROM (
		SELECT id, name, 'personal' AS kind, ord AS ord, 0 AS grp FROM labels
		UNION
		SELECT '' AS id, label_name AS name, 'shared' AS kind, 0 AS ord, 1 AS grp
			FROM task_labels
			WHERE label_name NOT IN (SELECT name FROM labels)
	)
	ORDER BY grp, ord, name`

func init() {
	labelsRmCmd.Flags().BoolVarP(&labelsRmForce, "force", "f", false, "skip the confirmation prompt for labels in use")
	labelsRmCmd.Flags().BoolVar(&labelsRmUnused, "unused", false, "delete every label attached to no cached task")
	labelsCmd.AddCommand(labelsRmCmd)
	labelsCmd.AddCommand(labelsRenameCmd)
	root.AddCommand(labelsCmd)
}
