package camera

import (
	"context"
	"fmt"
	"time"
)

// Керування моторизованим об'єктивом (зум/фокус) через PTZ-continuous.
// Камера DS-2CD1743G0-IZ не має pan/tilt, але приймає continuous для зуму й фокуса.

// ptzContinuous надсилає команду безперервного руху (зум/фокус).
func (c *Client) ptzContinuous(ctx context.Context, zoom, focus int) error {
	body := fmt.Sprintf(
		`<PTZData><pan>0</pan><tilt>0</tilt><zoom>%d</zoom><focus>%d</focus></PTZData>`,
		zoom, focus)
	_, err := c.put(ctx, "/ISAPI/PTZCtrl/channels/1/continuous", []byte(body))
	return err
}

// lensPulse запускає рух об'єктива на тривалість d, потім зупиняє.
func (c *Client) lensPulse(ctx context.Context, zoom, focus int, d time.Duration) error {
	if err := c.ptzContinuous(ctx, zoom, focus); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
	return c.ptzContinuous(ctx, 0, 0) // стоп
}

const (
	lensSpeed    = 60
	lensPulseDur = 350 * time.Millisecond
)

// ZoomIn / ZoomOut — короткий крок зуму.
func (c *Client) ZoomIn(ctx context.Context) error  { return c.lensPulse(ctx, lensSpeed, 0, lensPulseDur) }
func (c *Client) ZoomOut(ctx context.Context) error { return c.lensPulse(ctx, -lensSpeed, 0, lensPulseDur) }

// FocusNear / FocusFar — короткий крок фокуса.
func (c *Client) FocusNear(ctx context.Context) error { return c.lensPulse(ctx, 0, lensSpeed, lensPulseDur) }
func (c *Client) FocusFar(ctx context.Context) error  { return c.lensPulse(ctx, 0, -lensSpeed, lensPulseDur) }

// GetFlip повертає поточний стан перевороту зображення.
func (c *Client) GetFlip(ctx context.Context) (bool, error) {
	body, err := c.get(ctx, "/ISAPI/Image/channels/1/ImageFlip")
	if err != nil {
		return false, err
	}
	m := enabledRe.FindSubmatch(body)
	return m != nil && string(m[1]) == "true", nil
}

// SetFlip вмикає/вимикає переворот зображення (на 180°).
func (c *Client) SetFlip(ctx context.Context, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	body := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<ImageFlip version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">`+
			`<enabled>%s</enabled><ImageFlipStyle>CENTER</ImageFlipStyle></ImageFlip>`, val)
	resp, err := c.put(ctx, "/ISAPI/Image/channels/1/ImageFlip", []byte(body))
	if err != nil {
		return fmt.Errorf("%w (%s)", err, string(resp))
	}
	return nil
}

// ToggleFlip перемикає переворот і повертає новий стан.
func (c *Client) ToggleFlip(ctx context.Context) (bool, error) {
	cur, err := c.GetFlip(ctx)
	if err != nil {
		return false, err
	}
	if err := c.SetFlip(ctx, !cur); err != nil {
		return false, err
	}
	return !cur, nil
}
