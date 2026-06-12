// Package capture — періодичне збереження знімків з камери.
package capture

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"nvr/internal/camera"
	"nvr/internal/store"
)

type Service struct {
	cam      camera.Camera
	store    *store.Store
	dataDir  string
	interval time.Duration
}

func New(cam camera.Camera, st *store.Store, dataDir string, interval time.Duration) *Service {
	return &Service{cam: cam, store: st, dataDir: dataDir, interval: interval}
}

// Run робить знімок одразу, далі — кожні interval, доки ctx не скасують.
func (s *Service) Run(ctx context.Context) {
	log.Printf("capture: знімки кожні %s", s.interval)
	if _, err := s.Capture(ctx, "schedule"); err != nil {
		log.Printf("capture: перший знімок: %v", err)
	}
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("capture: зупинено")
			return
		case <-t.C:
			if _, err := s.Capture(ctx, "schedule"); err != nil {
				log.Printf("capture: %v", err)
			}
		}
	}
}

// Capture робить один знімок, зберігає на диск і в БД. Повертає шлях.
func (s *Service) Capture(ctx context.Context, trigger string) (string, error) {
	jpg, err := s.cam.Snapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("знімок: %w", err)
	}
	now := time.Now()
	dir := filepath.Join(s.dataDir, "snapshots", now.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, now.Format("15-04-05")+".jpg")
	if err := os.WriteFile(path, jpg, 0o644); err != nil {
		return "", err
	}
	if _, err := s.store.AddSnapshot(store.Snapshot{
		Path:    path,
		TakenAt: now,
		Size:    int64(len(jpg)),
		Trigger: trigger,
	}); err != nil {
		log.Printf("capture: запис у БД: %v", err)
	}
	log.Printf("capture: %s (%d КБ, %s)", path, len(jpg)/1024, trigger)
	return path, nil
}
