// Package ha calls the Home Assistant REST API.
package ha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL, Token string
	HTTP           *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify calls notify.<service> for a target of the form "notify.<service>".
func (c *Client) Notify(ctx context.Context, target, title, message string, data map[string]any) error {
	service, ok := strings.CutPrefix(target, "notify.")
	if !ok || service == "" {
		return fmt.Errorf("target %q must look like notify.<service>", target)
	}
	body := map[string]any{"title": title, "message": message}
	if data != nil {
		body["data"] = data
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/api/services/notify/"+service, bytes.NewReader(payload))
}

// Ping checks that the URL and token are accepted.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/", nil)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach Home Assistant at %s: %s", c.BaseURL, redact(err.Error(), c.Token))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("HA returned %s: %s", resp.Status, redact(strings.TrimSpace(string(b)), c.Token))
	}
	return nil
}

func redact(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}
