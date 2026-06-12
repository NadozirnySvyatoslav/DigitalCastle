// Package recorder записує відеокліпи з RTSP-потоку камери через ffmpeg.
package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Recorder struct {
	rtspURL string
	dataDir string
}

func New(rtspURL, dataDir string) *Recorder {
	return &Recorder{rtspURL: rtspURL, dataDir: dataDir}
}

// RecordClip записує seconds секунд у mp4 (копія потоку, без перекодування).
// Повертає шлях до файлу.
func (r *Recorder) RecordClip(ctx context.Context, seconds int) (string, error) {
	now := time.Now()
	dir := filepath.Join(r.dataDir, "recordings", now.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, now.Format("15-04-05")+".mp4")

	// окремий таймаут трохи більший за тривалість кліпу
	cctx, cancel := context.WithTimeout(ctx, time.Duration(seconds+15)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, "ffmpeg",
		"-y",
		"-rtsp_transport", "tcp",
		"-i", r.rtspURL,
		"-t", fmt.Sprintf("%d", seconds),
		"-c", "copy",
		"-movflags", "+faststart",
		path,
		"-loglevel", "error",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg: %w: %s", err, string(out))
	}
	return path, nil
}
