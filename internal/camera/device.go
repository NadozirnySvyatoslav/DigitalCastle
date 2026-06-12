package camera

import (
	"context"
	"encoding/xml"
)

// DeviceInfo — підмножина полів /ISAPI/System/deviceInfo.
type DeviceInfo struct {
	DeviceName      string `xml:"deviceName"`
	Model           string `xml:"model"`
	SerialNumber    string `xml:"serialNumber"`
	MACAddress      string `xml:"macAddress"`
	FirmwareVersion string `xml:"firmwareVersion"`
}

// DeviceInfo повертає інформацію про камеру.
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
