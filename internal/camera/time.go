package camera

import (
	"context"
	"fmt"
	"time"
)

// SyncTime виставляє годинник камери у ручному режимі за переданим часом.
// Корисно, коли камера без доступу до NTP (ізольована мережа) і після
// перезавантаження скидає час на 1970.
func (c *Client) SyncTime(ctx context.Context, t time.Time) error {
	// зсув таймзони у форматі +03:00
	_, offsetSec := t.Zone()
	sign := "+"
	if offsetSec < 0 {
		sign = "-"
		offsetSec = -offsetSec
	}
	offset := fmt.Sprintf("%s%02d:%02d", sign, offsetSec/3600, (offsetSec%3600)/60)
	// Hikvision-формат таймзони інвертований: UTC+3 => CST-3:00:00
	tzSign := "-"
	if sign == "-" {
		tzSign = "+"
	}
	tz := fmt.Sprintf("CST%s%d:%02d:00", tzSign, offsetSec/3600, (offsetSec%3600)/60)

	local := t.Format("2006-01-02T15:04:05") + offset
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Time version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<timeMode>manual</timeMode>
<localTime>%s</localTime>
<timeZone>%s</timeZone>
</Time>`, local, tz)

	resp, err := c.put(ctx, "/ISAPI/System/time", []byte(body))
	if err != nil {
		return fmt.Errorf("%w (%s)", err, string(resp))
	}
	return nil
}
