package main

import (
	"errors"
	"fmt"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

// todoistProjectNotFound is the API's error_code for a project that no longer
// exists server-side.
const todoistProjectNotFound = 478

// explainStaleProject turns the raw API "project not found" error into an
// actionable message pointing at the real cause — a stale local cache — and the
// one command that fixes it. Any other error is returned unchanged.
func explainStaleProject(err error, projectName string) error {
	var apiErr *todoist.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode == todoistProjectNotFound {
		return fmt.Errorf("project %q no longer exists on the server — "+
			"your local cache may be stale, run: todoist-cli sync", projectName)
	}
	return err
}
