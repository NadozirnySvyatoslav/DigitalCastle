package camera

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// alertNotification відображає XML <EventNotificationAlert>.
type alertNotification struct {
	EventType        string `xml:"eventType"`
	EventState       string `xml:"eventState"`
	EventDescription string `xml:"eventDescription"`
}

// WatchMotion підписується на потік подій камери і викликає onMotion для
// кожної активної події руху (VMD). Блокує до скасування ctx; при обриві
// з'єднання автоматично перепідключається.
func (c *Client) WatchMotion(ctx context.Context, onMotion func(MotionEvent)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := c.streamAlerts(ctx, onMotion); err != nil && ctx.Err() == nil {
			log.Printf("motion: потік обірвано (%v), перепідключення через 3с", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}
}

func (c *Client) streamAlerts(ctx context.Context, onMotion func(MotionEvent)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/ISAPI/Event/notification/alertStream", nil)
	if err != nil {
		return err
	}
	resp, err := c.stream.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alertStream: HTTP %d", resp.StatusCode)
	}

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return fmt.Errorf("alertStream: неочікуваний Content-Type %q", resp.Header.Get("Content-Type"))
	}

	mr := multipart.NewReader(resp.Body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		body, err := io.ReadAll(part)
		part.Close()
		if err != nil {
			return err
		}
		var a alertNotification
		if xml.Unmarshal(body, &a) != nil {
			continue
		}
		// VMD active = рух. Інші типи (videoloss/inactive) — heartbeat, ігноруємо.
		if a.EventType == "VMD" && a.EventState == "active" {
			onMotion(MotionEvent{
				Type:        a.EventType,
				State:       a.EventState,
				Description: a.EventDescription,
				At:          time.Now(),
			})
		}
	}
}
