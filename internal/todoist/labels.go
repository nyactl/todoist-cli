package todoist

import (
	"context"
	"net/url"
)

func (c *Client) GetLabels(ctx context.Context) ([]Label, error) {
	return queryAll[Label](c, ctx, "/labels", url.Values{})
}

// UpdateLabel renames (or otherwise updates) a label. Todoist propagates a
// rename to every task carrying the label server-side, so callers only need to
// mirror the change into the local cache.
func (c *Client) UpdateLabel(ctx context.Context, id string, req UpdateLabelRequest) (*Label, error) {
	var l Label
	return &l, c.doJSON(ctx, "POST", "/labels/"+id, req, &l)
}

// DeleteLabel deletes a personal label; the server detaches it from every task.
func (c *Client) DeleteLabel(ctx context.Context, id string) error {
	resp, err := c.do(ctx, "DELETE", "/labels/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RenameSharedLabel renames a shared label (one that lives only as a name on
// tasks, not in the personal label list) across every task carrying it.
func (c *Client) RenameSharedLabel(ctx context.Context, name, newName string) error {
	resp, err := c.do(ctx, "POST", "/labels/shared/rename",
		map[string]string{"name": name, "new_name": newName})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RemoveSharedLabel removes a shared label from every task carrying it.
func (c *Client) RemoveSharedLabel(ctx context.Context, name string) error {
	resp, err := c.do(ctx, "POST", "/labels/shared/remove",
		map[string]string{"name": name})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
