package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/nyactl/todoist-cli/internal/state"
	"github.com/nyactl/todoist-cli/internal/tasks"
)

// loadContext reads the active project context and validates it against the
// local cache. If the project no longer exists it auto-clears and warns.
func loadContext(ctx context.Context, db *sql.DB) (*state.State, error) {
	s, err := state.Load()
	if err != nil || !s.HasProject() {
		return s, err
	}
	exists, err := tasks.ProjectExists(ctx, db, s.ProjectID)
	if err != nil {
		return nil, err
	}
	if !exists {
		fmt.Fprintf(os.Stderr,
			"warning: project %q no longer exists in local cache — context cleared\n"+
				"         run: todoist-cli sync\n",
			s.ProjectName)
		if err := state.Clear(); err != nil {
			return nil, err
		}
		return &state.State{}, nil
	}
	return s, nil
}
