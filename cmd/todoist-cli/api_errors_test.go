package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nyactl/todoist-cli/internal/todoist"
)

func TestExplainStaleProject(t *testing.T) {
	rawBody := `{"error":"Project not found","error_code":478,"error_extra":{"event_id":"abc","retry_after":3},"error_tag":"NOT_FOUND","http_code":404}`
	notFound := &todoist.APIError{StatusCode: 404, Body: rawBody, ErrorCode: 478, ErrorTag: "NOT_FOUND"}

	tests := []struct {
		name        string
		in          error
		project     string
		wantChanged bool // true = translated to the actionable message
	}{
		{"project-not-found translated", notFound, "Archive", true},
		{"wrapped project-not-found translated", fmt.Errorf("create task: %w", notFound), "Archive", true},
		{"other api error untouched", &todoist.APIError{StatusCode: 400, Body: "bad", ErrorCode: 400}, "Archive", false},
		{"plain error untouched", errors.New("network down"), "Archive", false},
		{"nil stays nil", nil, "Archive", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := explainStaleProject(tc.in, tc.project)
			if tc.in == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if tc.wantChanged {
				msg := got.Error()
				if !strings.Contains(msg, tc.project) {
					t.Errorf("expected project name %q in message, got: %q", tc.project, msg)
				}
				if !strings.Contains(msg, "todoist-cli sync") {
					t.Errorf("expected sync hint in message, got: %q", msg)
				}
				// The raw transport blob must not leak through.
				if strings.Contains(msg, "event_id") || strings.Contains(msg, "error_extra") {
					t.Errorf("raw API blob leaked into message: %q", msg)
				}
			} else if got != tc.in {
				t.Errorf("expected error returned unchanged, got: %v", got)
			}
		})
	}
}
