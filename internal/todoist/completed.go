package todoist

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// completedWindow stays below the API's three-month cap on since..until for
// every calendar span (three months can be as short as 89 days).
const completedWindow = 85 * 24 * time.Hour

// completedPageSize is the API's maximum page size.
const completedPageSize = 200

// GetCompleted returns the tasks completed between since and until, newest
// window first, optionally only those in projectID. Longer spans are fetched
// in windows the API accepts, and every page of each window is followed.
func (c *Client) GetCompleted(ctx context.Context, since, until time.Time, projectID string) ([]CompletedTask, error) {
	var all []CompletedTask
	// Adjacent windows share their boundary instant, so a completion at that
	// instant can come back twice. A recurring task legitimately repeats its
	// ID, hence the key includes the completion time.
	seen := map[[2]string]bool{}
	for end := until; end.After(since); end = end.Add(-completedWindow) {
		start := end.Add(-completedWindow)
		if start.Before(since) {
			start = since
		}
		items, err := c.completedIn(ctx, start, end, projectID)
		if err != nil {
			return nil, err
		}
		for _, t := range items {
			key := [2]string{t.TaskID, t.CompletedAt}
			if !seen[key] {
				seen[key] = true
				all = append(all, t)
			}
		}
	}
	return all, nil
}

func (c *Client) completedIn(ctx context.Context, since, until time.Time, projectID string) ([]CompletedTask, error) {
	var all []CompletedTask
	cursor := ""
	for {
		params := url.Values{
			"since": {since.UTC().Format(time.RFC3339)},
			"until": {until.UTC().Format(time.RFC3339)},
			"limit": {strconv.Itoa(completedPageSize)},
		}
		if projectID != "" {
			params.Set("project_id", projectID)
		}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		var pg struct {
			Items      []CompletedTask `json:"items"`
			NextCursor string          `json:"next_cursor"`
		}
		if err := c.doQuery(ctx, "/tasks/completed/by_completion_date", params, &pg); err != nil {
			return nil, err
		}
		all = append(all, pg.Items...)
		if pg.NextCursor == "" {
			return all, nil
		}
		cursor = pg.NextCursor
	}
}
