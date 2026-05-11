package todoist

import (
	"context"
	"net/url"
	"time"
)

type completedPage struct {
	Items    []CompletedTask       `json:"items"`
	Projects map[string]namedEntry `json:"projects"`
	Sections map[string]namedEntry `json:"sections"`
}

type namedEntry struct {
	Name string `json:"name"`
}

func (c *Client) GetCompletedSince(ctx context.Context, since time.Time, projectID string) (*CompletedResult, error) {
	params := url.Values{"since": {since.UTC().Format(time.RFC3339)}}
	if projectID != "" {
		params.Set("project_id", projectID)
	}
	var pg completedPage
	if err := c.doQuery(ctx, "/tasks/completed", params, &pg); err != nil {
		return nil, err
	}
	res := &CompletedResult{
		Tasks:       pg.Items,
		ProjectName: make(map[string]string, len(pg.Projects)),
		SectionName: make(map[string]string, len(pg.Sections)),
	}
	for id, p := range pg.Projects {
		res.ProjectName[id] = p.Name
	}
	for id, s := range pg.Sections {
		res.SectionName[id] = s.Name
	}
	return res, nil
}
