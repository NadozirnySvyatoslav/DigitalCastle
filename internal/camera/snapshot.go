package camera

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Snapshot повертає JPEG-кадр з головного каналу (channel 101).
func (c *Client) Snapshot(ctx context.Context) ([]byte, error) {
	return c.snapshotChannel(ctx, 101)
}

func (c *Client) snapshotChannel(ctx context.Context, channel int) ([]byte, error) {
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d/picture", channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("snapshot: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
