package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const defaultBaseURL = "https://api.todoist.com/api/v1"

func apiBaseURL() string {
	if base := os.Getenv("TODOIST_API_BASE"); base != "" {
		return base
	}
	return defaultBaseURL
}

type Client struct {
	token string
	http  *http.Client
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("todoist: HTTP %d: %s", e.StatusCode, e.Body)
}

func New(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBaseURL()+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}
	return resp, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil || resp.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) doQuery(ctx context.Context, path string, params url.Values, out any) error {
	full := apiBaseURL() + path
	if len(params) > 0 {
		full += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func queryAll[T any](c *Client, ctx context.Context, path string, params url.Values) ([]T, error) {
	type page struct {
		Results    []T    `json:"results"`
		NextCursor string `json:"next_cursor"`
	}
	var all []T
	cursor := ""
	for {
		p := url.Values{}
		for k, v := range params {
			p[k] = v
		}
		if cursor != "" {
			p.Set("cursor", cursor)
		}
		var pg page
		if err := c.doQuery(ctx, path, p, &pg); err != nil {
			return nil, err
		}
		all = append(all, pg.Results...)
		if pg.NextCursor == "" {
			break
		}
		cursor = pg.NextCursor
	}
	return all, nil
}
