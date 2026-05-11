package todoist

import (
	"context"
	"net/url"
)

func (c *Client) GetComments(ctx context.Context, taskID string) ([]Comment, error) {
	return queryAll[Comment](c, ctx, "/comments", url.Values{"task_id": {taskID}})
}
