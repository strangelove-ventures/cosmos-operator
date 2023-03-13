package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

// Client can be used to query healthcheck information.
type Client struct {
	httpDo func(req *http.Request) (*http.Response, error)
}

func NewClient(client *http.Client) *Client {
	return &Client{
		httpDo: client.Do,
	}
}

// DiskUsage returns disk usage statistics or an error if unable to obtain.
// Do not include the port in the host.
func (c Client) DiskUsage(ctx context.Context, host string) (DiskUsageResponse, error) {
	var diskResp DiskUsageResponse
	u, err := url.Parse(host)
	if err != nil {
		return diskResp, fmt.Errorf("url parse: %w", err)
	}
	u.Host = net.JoinHostPort(u.Host, strconv.Itoa(Port))
	u.Path = "/disk"

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
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
	if diskResp.AllBytes == 0 {
		return diskResp, errors.New("invalid response: 0 free bytes")
	}
	return diskResp, nil
}
