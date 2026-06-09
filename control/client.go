package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	runtime "github.com/akula410/runtime"
)

// Client is an HTTP client for the control server.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient creates a Client targeting the given base URL (e.g. "http://127.0.0.1:7070").
// token is the optional bearer token (empty = no auth).
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Status returns the application status.
func (c *Client) Status(ctx context.Context) (runtime.AppStatus, error) {
	var out runtime.AppStatus
	return out, c.get(ctx, "/status", &out)
}

// Stop requests the application to shut down.
func (c *Client) Stop(ctx context.Context) error {
	return c.post(ctx, "/stop", nil)
}

// ServiceStatus returns the status of a named service.
func (c *Client) ServiceStatus(ctx context.Context, name string) (runtime.HealthStatus, error) {
	var out runtime.HealthStatus
	return out, c.get(ctx, "/services/"+name+"/status", &out)
}

// StartService starts the named service.
func (c *Client) StartService(ctx context.Context, name string) error {
	return c.post(ctx, "/services/"+name+"/start", nil)
}

// StopService stops the named service.
func (c *Client) StopService(ctx context.Context, name string) error {
	return c.post(ctx, "/services/"+name+"/stop", nil)
}

// RestartService restarts the named service.
func (c *Client) RestartService(ctx context.Context, name string) error {
	return c.post(ctx, "/services/"+name+"/restart", nil)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, out)
}

func (c *Client) post(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodPost, path, out)
}

func (c *Client) do(ctx context.Context, method, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("control client: build request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("control client: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return fmt.Errorf("control client: %s %s: %s", method, path, e.Error)
		}
		return fmt.Errorf("control client: %s %s: HTTP %d", method, path, resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("control client: decode response: %w", err)
		}
	}
	return nil
}
