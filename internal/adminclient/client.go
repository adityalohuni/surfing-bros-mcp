package adminclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/adityalohuni/mcp-server/internal/admin"
	"github.com/adityalohuni/mcp-server/internal/session"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    httpClient,
	}
}

func (c *Client) ListClients(ctx context.Context) ([]session.ClientInfo, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/admin/clients")
	if err != nil {
		return nil, err
	}
	var out []session.ClientInfo
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListBrowsers(ctx context.Context) ([]admin.BrowserSession, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/admin/browsers")
	if err != nil {
		return nil, err
	}
	var out []admin.BrowserSession
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DisconnectClient(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/admin/clients/disconnect?id="+url.QueryEscape(id))
	if err != nil {
		return err
	}
	return c.doNoBody(req)
}

func (c *Client) DisconnectBrowser(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodPost, "/admin/browsers/disconnect?id="+url.QueryEscape(id))
	if err != nil {
		return err
	}
	return c.doNoBody(req)
}

func (c *Client) newRequest(ctx context.Context, method, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return req, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("admin request failed: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func (c *Client) doNoBody(req *http.Request) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("admin request failed: %s", resp.Status)
	}
	return nil
}
