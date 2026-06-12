package camera

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// generic — драйвер для будь-якої RTSP-камери. Знімки й детектор руху — через ffmpeg.
type generic struct {
	rtspMain  string
	rtspSub   string
	threshold float64 // поріг детекції сцени (0..1)
}

// defaultSceneThreshold — поміркований дефолт (спрацьовує на появу об'єкта в кадрі).
const defaultSceneThreshold = 0.05

func newGeneric(main, sub string, threshold float64) *generic {
	if threshold <= 0 {
		threshold = defaultSceneThreshold
	}
	return &generic{rtspMain: main, rtspSub: sub, threshold: threshold}
}

func (g *generic) RTSPMain() string { return g.rtspMain }

func (g *generic) Capabilities() Capabilities {
	return Capabilities{
		Driver:         "generic",
		Lens:           false,
		Flip:           false,
		HardwareMotion: false, // детектор руху програмний (ffmpeg)
		MotionConfig:   false,
		TimeSync:       false,
	}
}

// DeviceInfo пробує дізнатись роздільність/кодек через ffprobe.
func (g *generic) DeviceInfo(ctx context.Context) (*DeviceInfo, error) {
	di := &DeviceInfo{Model: "RTSP-камера", DeviceName: "Generic"}
	cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "ffprobe",
		"-rtsp_transport", "tcp", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height",
		"-of", "default=noprint_wrappers=1:nokey=1",
		g.rtspMain,
	).Output()
	if err == nil {
		f := strings.Fields(string(out))
		if len(f) >= 3 {
			di.Model = fmt.Sprintf("RTSP %sx%s %s", f[1], f[2], strings.ToUpper(f[0]))
			di.FirmwareVersion = "generic"
		}
	}
	return di, nil
}

// Snapshot бере один кадр з головного RTSP-потоку через ffmpeg.
func (g *generic) Snapshot(ctx context.Context) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", g.rtspMain,
		"-frames:v", "1",
		"-q:v", "3",
		"-f", "mjpeg",
		"pipe:1",
		"-loglevel", "error",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg snapshot: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ffmpeg snapshot: порожній кадр")
	}
	return out, nil
}

// MotionDetectionEnabled — програмний детектор завжди доступний.
func (g *generic) MotionDetectionEnabled(ctx context.Context) (bool, error) { return true, nil }

// SetMotionDetection — no-op (детектор програмний, керується самим NVR).
func (g *generic) SetMotionDetection(ctx context.Context, enabled bool) error { return nil }

// WatchMotion аналізує підпотік через ffmpeg (детекція зміни сцени) і викликає
// onMotion при перевищенні порога. Блокує до скасування ctx; перепідключається.
func (g *generic) WatchMotion(ctx context.Context, onMotion func(MotionEvent)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := g.detectMotion(ctx, onMotion); err != nil && ctx.Err() == nil {
			log.Printf("motion(generic): %v, перепідключення через 3с", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}
}

func (g *generic) detectMotion(ctx context.Context, onMotion func(MotionEvent)) error {
	vf := fmt.Sprintf("select='gt(scene,%.3f)',metadata=print", g.threshold)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", g.rtspSub,
		"-an",
		"-vf", vf,
		"-f", "null", "-",
		"-loglevel", "info",
	)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stderr)
	for sc.Scan() {
		// рядок виду: "... lavfi.scene_score=0.1234" — кадр перевищив поріг = рух
		if strings.Contains(sc.Text(), "lavfi.scene_score") {
			onMotion(MotionEvent{
				Type:        "VMD",
				State:       "active",
				Description: "Software motion (scene change)",
				At:          time.Now(),
			})
		}
	}
	return cmd.Wait()
}

// --- функції об'єктива/часу: generic-камера не підтримує ---

func (g *generic) ZoomIn(ctx context.Context) error            { return ErrUnsupported }
func (g *generic) ZoomOut(ctx context.Context) error           { return ErrUnsupported }
func (g *generic) FocusNear(ctx context.Context) error         { return ErrUnsupported }
func (g *generic) FocusFar(ctx context.Context) error          { return ErrUnsupported }
func (g *generic) GetFlip(ctx context.Context) (bool, error)   { return false, ErrUnsupported }
func (g *generic) SetFlip(ctx context.Context, e bool) error   { return ErrUnsupported }
func (g *generic) ToggleFlip(ctx context.Context) (bool, error) { return false, ErrUnsupported }
func (g *generic) SyncTime(ctx context.Context, t time.Time) error { return ErrUnsupported }
