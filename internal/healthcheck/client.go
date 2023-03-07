package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Client can be used to query healthcheck information.
type Client struct {
	rootURL string
	httpDo  func(req *http.Request) (*http.Response, error)
}

func NewClient(client *http.Client) *Client {
	return &Client{
		rootURL: fmt.Sprintf("http://localhost:%d", Port),
		httpDo:  client.Do,
	}
}

// DiskUsage returns disk usage statistics or an error if unable to obtain.
func (c Client) DiskUsage(ctx context.Context) (DiskUsageResponse, error) {
	var diskResp DiskUsageResponse
	req, err := http.NewRequestWithContext(ctx, "GET", c.rootURL+"/disk", nil)
	if err != nil {
		return diskResp, fmt.Errorf("new request: %w", err)
	}
	resp, err := c.httpDo(req)
	if err != nil {
		return diskResp, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&diskResp); err != nil {
		return diskResp, fmt.Errorf("malformed json: %w", err)
	}
	if diskResp.Error != "" {
		return diskResp, errors.New(diskResp.Error)
	}
	return diskResp, nil
}
