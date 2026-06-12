// Package bot — Telegram-бот для керування камерою та сповіщень.
package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// api — мінімальний клієнт Telegram Bot API.
type api struct {
	token string
	http  *http.Client
}

func newAPI(token string) *api {
	return &api{token: token, http: &http.Client{Timeout: 60 * time.Second}}
}

func (a *api) url(method string) string {
	return "https://api.telegram.org/bot" + a.token + "/" + method
}

// Update — вхідне оновлення (повідомлення/команда/натискання кнопки).
type Update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		From      struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		From struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Message *struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"callback_query"`
}

// InlineButton — кнопка inline-клавіатури.
type InlineButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

// getUpdates робить long-polling від offset.
func (a *api) getUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	q := url.Values{}
	q.Set("offset", strconv.FormatInt(offset, 10))
	q.Set("timeout", strconv.Itoa(timeoutSec))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url("getUpdates")+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if !r.OK {
		return nil, fmt.Errorf("getUpdates: ok=false")
	}
	return r.Result, nil
}

func (a *api) sendMessage(ctx context.Context, chatID int64, text string) error {
	q := url.Values{}
	q.Set("chat_id", strconv.FormatInt(chatID, 10))
	q.Set("text", text)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url("sendMessage"),
		bytes.NewBufferString(q.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// sendMessageWithButtons надсилає текст з inline-клавіатурою (кожна кнопка — окремий рядок).
func (a *api) sendMessageWithButtons(ctx context.Context, chatID int64, text string, buttons []InlineButton) error {
	rows := make([][]InlineButton, len(buttons))
	for i, b := range buttons {
		rows[i] = []InlineButton{b}
	}
	markup, _ := json.Marshal(map[string]any{"inline_keyboard": rows})
	q := url.Values{}
	q.Set("chat_id", strconv.FormatInt(chatID, 10))
	q.Set("text", text)
	q.Set("reply_markup", string(markup))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url("sendMessage"),
		bytes.NewBufferString(q.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// answerCallback закриває "годинник" на натиснутій кнопці.
func (a *api) answerCallback(ctx context.Context, callbackID string) {
	q := url.Values{}
	q.Set("callback_query_id", callbackID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.url("answerCallbackQuery"),
		bytes.NewBufferString(q.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := a.http.Do(req); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// sendPhoto надсилає JPEG-байти з підписом.
func (a *api) sendPhoto(ctx context.Context, chatID int64, jpg []byte, caption string) error {
	return a.sendFile(ctx, "sendPhoto", "photo", "snapshot.jpg", chatID, jpg, caption)
}

// sendVideo надсилає mp4-байти з підписом.
func (a *api) sendVideo(ctx context.Context, chatID int64, mp4 []byte, caption string) error {
	return a.sendFile(ctx, "sendVideo", "video", "clip.mp4", chatID, mp4, caption)
}

func (a *api) sendFile(ctx context.Context, method, field, filename string, chatID int64, data []byte, caption string) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	if caption != "" {
		w.WriteField("caption", caption)
	}
	fw, err := w.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	if _, err := fw.Write(data); err != nil {
		return err
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url(method), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d: %s", method, resp.StatusCode, string(body))
	}
	return nil
}
