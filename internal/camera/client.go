// Package camera — клієнт до IP-камери Hikvision через ISAPI (HTTP Digest).
package camera

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/icholy/digest"
)

// Client спілкується з камерою по ISAPI.
type Client struct {
	baseURL string
	http    *http.Client // звичайні запити (з таймаутом)
	stream  *http.Client // довгоживучі потоки (без таймауту, керується через ctx)
}

// New створює клієнт. host — IP або хост камери (без схеми).
func New(host, username, password string) *Client {
	mkTransport := func() *digest.Transport {
		return &digest.Transport{Username: username, Password: password}
	}
	return &Client{
		baseURL: "http://" + host,
		http: &http.Client{
			Timeout:   15 * time.Second,
			Transport: mkTransport(),
		},
		stream: &http.Client{
			// без Timeout: alertStream — нескінченний потік;
			// тривалість керується через context у викликах
			Transport: mkTransport(),
		},
	}
}

// get виконує GET ISAPI-запит і повертає тіло відповіді.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	return body, nil
}

// put виконує PUT ISAPI-запит з XML-тілом.
func (c *Client) put(ctx context.Context, path string, xmlBody []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, bytes.NewReader(xmlBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PUT %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("PUT %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}
	return body, nil
}
