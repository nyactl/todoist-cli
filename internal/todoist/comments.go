package todoist

import (
	"context"
	"net/url"
)

func (c *Client) GetComments(ctx context.Context, taskID string) ([]Comment, error) {
	return queryAll[Comment](c, ctx, "/comments", url.Values{"task_id": {taskID}})
}

// GetCollaborators lists the members of a project, used to resolve comment
// authors (posted_uid) to display names.
func (c *Client) GetCollaborators(ctx context.Context, projectID string) ([]Collaborator, error) {
	return queryAll[Collaborator](c, ctx, "/projects/"+projectID+"/collaborators", url.Values{})
}

func (c *Client) PostComment(ctx context.Context, taskID, content string) (*Comment, error) {
	var comment Comment
	return &comment, c.doJSON(ctx, "POST", "/comments", map[string]string{
		"task_id": taskID,
		"content": content,
	}, &comment)
}
