package todoist

import (
	"context"
	"net/url"
)

func (c *Client) GetLabels(ctx context.Context) ([]Label, error) {
	return queryAll[Label](c, ctx, "/labels", url.Values{})
}
