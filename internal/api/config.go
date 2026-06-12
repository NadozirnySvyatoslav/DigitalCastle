package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"nvr/internal/config"
)

// configDTO — плоске представлення налаштувань для форми у вебі.
type configDTO struct {
	Camera struct {
		Host     string `json:"host"`
		Username string `json:"username"`
		Password string `json:"password"`
		RTSPMain string `json:"rtsp_main"`
		RTSPSub  string `json:"rtsp_sub"`
	} `json:"camera"`
	Telegram struct {
		Token          string  `json:"token"`
		ChatID         int64   `json:"chat_id"`
		AllowedChats   []int64 `json:"allowed_chats"`
		NotifyOnMotion bool    `json:"notify_on_motion"`
	} `json:"telegram"`
	Capture struct {
		SnapshotInterval  string `json:"snapshot_interval"`
		DataDir           string `json:"data_dir"`
		MotionClipSeconds int    `json:"motion_clip_seconds"`
	} `json:"capture"`
	Retention struct {
		MaxDays   int     `json:"max_days"`
		MaxSizeGB float64 `json:"max_size_gb"`
		Interval  string  `json:"interval"`
	} `json:"retention"`
	Server struct {
		Addr string `json:"addr"`
	} `json:"server"`
}

func toDTO(c *config.Config) configDTO {
	var d configDTO
	d.Camera.Host = c.Camera.Host
	d.Camera.Username = c.Camera.Username
	d.Camera.Password = c.Camera.Password
	d.Camera.RTSPMain = c.Camera.RTSPMain
	d.Camera.RTSPSub = c.Camera.RTSPSub
	d.Telegram.Token = c.Telegram.Token
	d.Telegram.ChatID = c.Telegram.ChatID
	d.Telegram.AllowedChats = c.Telegram.AllowedChats
	d.Telegram.NotifyOnMotion = c.Telegram.NotifyOnMotion
	d.Capture.SnapshotInterval = c.Capture.SnapshotIntervalS
	d.Capture.DataDir = c.Capture.DataDir
	d.Capture.MotionClipSeconds = c.Capture.MotionClipSeconds
	d.Retention.MaxDays = c.Retention.MaxDays
	d.Retention.MaxSizeGB = c.Retention.MaxSizeGB
	d.Retention.Interval = c.Retention.IntervalS
	d.Server.Addr = c.Server.Addr
	return d
}

// toConfig перетворює DTO у config.Config з валідацією тривалостей.
func (d configDTO) toConfig() (*config.Config, error) {
	if d.Camera.Host == "" {
		return nil, fmt.Errorf("адреса камери (host) не може бути порожньою")
	}
	snap, err := time.ParseDuration(d.Capture.SnapshotInterval)
	if err != nil {
		return nil, fmt.Errorf("інтервал знімків %q: %w", d.Capture.SnapshotInterval, err)
	}
	if snap <= 0 {
		return nil, fmt.Errorf("інтервал знімків має бути додатним")
	}
	retInterval := "1h"
	if d.Retention.Interval != "" {
		if _, err := time.ParseDuration(d.Retention.Interval); err != nil {
			return nil, fmt.Errorf("інтервал прибирання %q: %w", d.Retention.Interval, err)
		}
		retInterval = d.Retention.Interval
	}
	if d.Server.Addr == "" {
		return nil, fmt.Errorf("адреса сервера не може бути порожньою")
	}

	c := &config.Config{}
	c.Camera.Host = d.Camera.Host
	c.Camera.Username = d.Camera.Username
	c.Camera.Password = d.Camera.Password
	c.Camera.RTSPMain = d.Camera.RTSPMain
	c.Camera.RTSPSub = d.Camera.RTSPSub
	c.Telegram.Token = d.Telegram.Token
	c.Telegram.ChatID = d.Telegram.ChatID
	c.Telegram.AllowedChats = d.Telegram.AllowedChats
	c.Telegram.NotifyOnMotion = d.Telegram.NotifyOnMotion
	c.Capture.SnapshotIntervalS = d.Capture.SnapshotInterval
	c.Capture.SnapshotInterval = snap
	c.Capture.DataDir = d.Capture.DataDir
	c.Capture.MotionClipSeconds = d.Capture.MotionClipSeconds
	c.Retention.MaxDays = d.Retention.MaxDays
	c.Retention.MaxSizeGB = d.Retention.MaxSizeGB
	c.Retention.IntervalS = retInterval
	c.Server.Addr = d.Server.Addr
	return c, nil
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, toDTO(s.cfg))
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var dto configDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "невірний JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	newCfg, err := dto.toConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.Save(s.configPath, newCfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.cfg = newCfg // оновлюємо в пам'яті; повне застосування — після перезапуску
	log.Printf("api: налаштування збережено у %s", s.configPath)
	writeJSON(w, map[string]any{"ok": true, "restart_required": true})
}

// handleRestart перезапускає сам процес (re-exec) — застосовує всі налаштування.
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"ok": true})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(700 * time.Millisecond)
		exe, err := os.Executable()
		if err != nil {
			log.Printf("api: restart: %v", err)
			return
		}
		log.Println("api: перезапуск сервісу за запитом з вебінтерфейсу...")
		if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
			log.Printf("api: re-exec: %v", err)
		}
	}()
}
