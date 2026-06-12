// Package llm — клієнт до Azure OpenAI (chat completions) для розбору запитів.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client struct {
	chatURL string
	apiKey  string
	http    *http.Client
}

// NewFromEnv будує клієнт із змінних оточення (.env):
//
//	AZURE_API_KEY  — ключ (обов'язково)
//	AZURE_ENDPOINT — будь-який URL ресурсу (беремо лише базовий домен)
//	AZURE_CHAT_DEPLOYMENT (необов'язково, типово gpt-4o-mini)
//	AZURE_API_VERSION     (необов'язково, типово 2024-08-01-preview)
func NewFromEnv() (*Client, error) {
	key := os.Getenv("AZURE_API_KEY")
	endpoint := os.Getenv("AZURE_ENDPOINT")
	if key == "" || endpoint == "" {
		return nil, fmt.Errorf("AZURE_API_KEY або AZURE_ENDPOINT не задані")
	}
	base := baseDomain(endpoint)
	deployment := envDefault("AZURE_CHAT_DEPLOYMENT", "gpt-4o-mini")
	version := envDefault("AZURE_API_VERSION", "2024-08-01-preview")
	return &Client{
		chatURL: fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
			base, deployment, version),
		apiKey: key,
		http:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// baseDomain лишає від URL лише схему+хост: https://x.azure.com/foo... -> https://x.azure.com
func baseDomain(u string) string {
	u = strings.TrimSpace(u)
	if i := strings.Index(u, "://"); i >= 0 {
		if j := strings.IndexByte(u[i+3:], '/'); j >= 0 {
			return u[:i+3+j]
		}
	}
	return strings.TrimRight(u, "/")
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type chatReq struct {
	Messages       []msg          `json:"messages"`
	Temperature    float64        `json:"temperature"`
	MaxTokens      int            `json:"max_tokens"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
}

type msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatJSON надсилає system+user і повертає текст відповіді у JSON-режимі.
func (c *Client) ChatJSON(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(chatReq{
		Messages: []msg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    0,
		MaxTokens:      500,
		ResponseFormat: map[string]any{"type": "json_object"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("azure: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("azure: порожня відповідь (HTTP %d)", resp.StatusCode)
	}
	return out.Choices[0].Message.Content, nil
}
