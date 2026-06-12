package camera

import (
	"context"
	"fmt"
	"regexp"
)

var enabledRe = regexp.MustCompile(`<enabled>(true|false)</enabled>`)

// MotionDetectionEnabled повертає поточний стан детектора руху.
func (c *Client) MotionDetectionEnabled(ctx context.Context) (bool, error) {
	body, err := c.get(ctx, "/ISAPI/System/Video/inputs/channels/1/motionDetection")
	if err != nil {
		return false, err
	}
	m := enabledRe.FindSubmatch(body)
	if m == nil {
		return false, fmt.Errorf("не знайдено <enabled> у відповіді")
	}
	return string(m[1]) == "true", nil
}

// SetMotionDetection вмикає/вимикає детектор руху, зберігаючи решту налаштувань
// (сітку, чутливість тощо) — читає поточний конфіг і змінює лише <enabled>.
func (c *Client) SetMotionDetection(ctx context.Context, enabled bool) error {
	body, err := c.get(ctx, "/ISAPI/System/Video/inputs/channels/1/motionDetection")
	if err != nil {
		return err
	}
	val := "false"
	if enabled {
		val = "true"
	}
	// замінюємо лише перший <enabled> (загальний прапорець детектора)
	replaced := enabledRe.ReplaceAll(body, []byte("<enabled>"+val+"</enabled>"))
	resp, err := c.put(ctx, "/ISAPI/System/Video/inputs/channels/1/motionDetection", replaced)
	if err != nil {
		return fmt.Errorf("%w (відповідь: %s)", err, string(resp))
	}
	return nil
}
