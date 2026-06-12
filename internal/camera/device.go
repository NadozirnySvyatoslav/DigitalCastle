package camera

import (
	"context"
	"encoding/xml"
)

// DeviceInfo повертає інформацію про камеру (поля з /ISAPI/System/deviceInfo).
func (c *Client) DeviceInfo(ctx context.Context) (*DeviceInfo, error) {
	body, err := c.get(ctx, "/ISAPI/System/deviceInfo")
	if err != nil {
		return nil, err
	}
	var di DeviceInfo
	if err := xml.Unmarshal(body, &di); err != nil {
		return nil, err
	}
	return &di, nil
}
