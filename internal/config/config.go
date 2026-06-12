// Package config завантажує налаштування NVR з YAML-файлу.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// LoadDotEnv читає простий .env (KEY=VALUE) і виставляє змінні оточення,
// не перезаписуючи вже наявні. Відсутній файл — не помилка.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
	return sc.Err()
}

type Config struct {
	Camera    Camera    `yaml:"camera"`
	Telegram  Telegram  `yaml:"telegram"`
	Capture   Capture   `yaml:"capture"`
	Retention Retention `yaml:"retention"`
	Server    Server    `yaml:"server"`
}

type Retention struct {
	MaxDays   int           `yaml:"max_days"`
	MaxSizeGB float64       `yaml:"max_size_gb"`
	Interval  time.Duration `yaml:"-"`
	IntervalS string        `yaml:"interval"`
}

type Camera struct {
	Type            string  `yaml:"type"` // "hikvision" (типово) | "generic"
	Host            string  `yaml:"host"`
	Username        string  `yaml:"username"`
	Password        string  `yaml:"password"`
	RTSPMain        string  `yaml:"rtsp_main"`
	RTSPSub         string  `yaml:"rtsp_sub"`
	MotionThreshold float64 `yaml:"motion_threshold"` // generic: поріг детекції руху 0..1 (типово 0.05)
}

type Telegram struct {
	Token          string  `yaml:"token"`
	ChatID         int64   `yaml:"chat_id"`       // основний чат (власник)
	AllowedChats   []int64 `yaml:"allowed_chats"` // додаткові дозволені чати (групи тощо)
	NotifyOnMotion bool    `yaml:"notify_on_motion"`
}

// Chats повертає всі дозволені чати (основний + додаткові, без дублів).
func (t Telegram) Chats() []int64 {
	seen := map[int64]bool{}
	var out []int64
	add := func(id int64) {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	add(t.ChatID)
	for _, id := range t.AllowedChats {
		add(id)
	}
	return out
}

type Capture struct {
	SnapshotInterval  time.Duration `yaml:"-"`
	SnapshotIntervalS string        `yaml:"snapshot_interval"`
	DataDir           string        `yaml:"data_dir"`
	MotionClipSeconds int           `yaml:"motion_clip_seconds"`
}

type Server struct {
	Addr string `yaml:"addr"`
}

// Save серіалізує конфіг назад у YAML-файл (права 0600 — містить секрети).
func Save(path string, c *Config) error {
	out, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("серіалізація YAML: %w", err)
	}
	header := []byte("# Згенеровано веб-інтерфейсом NVR. Можна редагувати й вручну.\n")
	if err := os.WriteFile(path, append(header, out...), 0o600); err != nil {
		return fmt.Errorf("запис %s: %w", path, err)
	}
	return nil
}

// Load читає та валідує конфіг із вказаного шляху.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("читання конфігу %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("розбір YAML: %w", err)
	}

	// перетворюємо рядок інтервалу на time.Duration
	d, err := time.ParseDuration(c.Capture.SnapshotIntervalS)
	if err != nil {
		return nil, fmt.Errorf("snapshot_interval %q: %w", c.Capture.SnapshotIntervalS, err)
	}
	c.Capture.SnapshotInterval = d

	switch c.Camera.Type {
	case "", "hikvision":
		if c.Camera.Host == "" {
			return nil, fmt.Errorf("camera.host порожній (потрібен для hikvision)")
		}
	default: // generic та інші — потрібен лише rtsp_main
		if c.Camera.RTSPMain == "" {
			return nil, fmt.Errorf("camera.rtsp_main порожній (потрібен для %s)", c.Camera.Type)
		}
	}
	if c.Capture.DataDir == "" {
		c.Capture.DataDir = "./data"
	}
	if c.Capture.MotionClipSeconds == 0 {
		c.Capture.MotionClipSeconds = 15
	}

	// інтервал прибирання (за замовчуванням 1 година)
	c.Retention.Interval = time.Hour
	if c.Retention.IntervalS != "" {
		d, err := time.ParseDuration(c.Retention.IntervalS)
		if err != nil {
			return nil, fmt.Errorf("retention.interval %q: %w", c.Retention.IntervalS, err)
		}
		c.Retention.Interval = d
	}
	return &c, nil
}
