// Package camera абстрагує IP-камеру за інтерфейсом Camera з кількома драйверами.
// Базовий драйвер "generic" працює з будь-якою RTSP-камерою (через ffmpeg);
// драйвер "hikvision" додає апаратні функції (детектор руху, зум/фокус, синх. часу).
package camera

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrUnsupported повертають драйвери для функцій, яких камера не має.
var ErrUnsupported = errors.New("не підтримується цією камерою")

// Capabilities описує, що вміє конкретна камера/драйвер.
type Capabilities struct {
	Driver         string `json:"driver"`          // "hikvision" | "generic"
	Lens           bool   `json:"lens"`            // зум/фокус
	Flip           bool   `json:"flip"`            // переворот зображення
	HardwareMotion bool   `json:"hardware_motion"` // апаратний детектор руху (інакше — програмний)
	MotionConfig   bool   `json:"motion_config"`   // можна вмикати/вимикати детектор на камері
	TimeSync       bool   `json:"time_sync"`       // синхронізація годинника камери
}

// MotionEvent — подія детектора руху (апаратного або програмного).
type MotionEvent struct {
	Type        string
	State       string
	Description string
	At          time.Time
}

// DeviceInfo — базова інформація про камеру.
type DeviceInfo struct {
	DeviceName      string `xml:"deviceName" json:"device_name"`
	Model           string `xml:"model" json:"model"`
	SerialNumber    string `xml:"serialNumber" json:"serial_number"`
	MACAddress      string `xml:"macAddress" json:"mac_address"`
	FirmwareVersion string `xml:"firmwareVersion" json:"firmware_version"`
}

// Camera — спільний інтерфейс для всіх драйверів камер.
type Camera interface {
	Capabilities() Capabilities
	DeviceInfo(ctx context.Context) (*DeviceInfo, error)
	Snapshot(ctx context.Context) ([]byte, error)
	RTSPMain() string

	// Детектор руху (завжди доступний: апаратний у hikvision, програмний у generic).
	WatchMotion(ctx context.Context, onMotion func(MotionEvent))
	MotionDetectionEnabled(ctx context.Context) (bool, error)
	SetMotionDetection(ctx context.Context, enabled bool) error

	// Керування об'єктивом (ErrUnsupported, якщо камера не вміє).
	ZoomIn(ctx context.Context) error
	ZoomOut(ctx context.Context) error
	FocusNear(ctx context.Context) error
	FocusFar(ctx context.Context) error
	GetFlip(ctx context.Context) (bool, error)
	SetFlip(ctx context.Context, enabled bool) error
	ToggleFlip(ctx context.Context) (bool, error)

	// Синхронізація часу камери (ErrUnsupported, якщо камера не вміє).
	SyncTime(ctx context.Context, t time.Time) error
}

// Options — параметри для створення камери.
type Options struct {
	Driver          string // "hikvision" | "generic" (типово hikvision для сумісності)
	Host            string
	Username        string
	Password        string
	RTSPMain        string
	RTSPSub         string
	MotionThreshold float64 // generic: поріг детекції руху (0 → дефолт)
}

// New створює камеру потрібного драйвера.
func New(o Options) (Camera, error) {
	switch o.Driver {
	case "", "hikvision":
		return newHikvision(o.Host, o.Username, o.Password, o.RTSPMain), nil
	case "generic", "rtsp", "onvif":
		main := o.RTSPMain
		sub := o.RTSPSub
		if sub == "" {
			sub = main
		}
		if main == "" {
			return nil, fmt.Errorf("generic-камера: потрібен rtsp_main")
		}
		return newGeneric(main, sub, o.MotionThreshold), nil
	default:
		return nil, fmt.Errorf("невідомий тип камери %q (допустимо: hikvision, generic)", o.Driver)
	}
}
