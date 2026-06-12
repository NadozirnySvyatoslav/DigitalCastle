// Package api — REST-сервер для React-фронтенду.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nvr/internal/camera"
	"nvr/internal/capture"
	"nvr/internal/config"
	"nvr/internal/recorder"
	"nvr/internal/store"
)

type Server struct {
	cfg        *config.Config
	configPath string
	cam        *camera.Client
	store      *store.Store
	cap        *capture.Service
	rec        *recorder.Recorder
}

func New(cfg *config.Config, configPath string, cam *camera.Client, st *store.Store, cap *capture.Service, rec *recorder.Recorder) *Server {
	return &Server{cfg: cfg, configPath: configPath, cam: cam, store: st, cap: cap, rec: rec}
}

// Handler збирає маршрути. webDir — тека зі зібраним React (може бути порожня).
func (s *Server) Handler(webDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/snapshot/live", s.handleLiveSnapshot)
	mux.HandleFunc("POST /api/snapshot", s.handleCapture)
	mux.HandleFunc("GET /api/snapshots", s.handleListSnapshots)
	mux.HandleFunc("GET /api/events", s.handleListEvents)
	mux.HandleFunc("POST /api/clip", s.handleClip)
	mux.HandleFunc("POST /api/lens/{action}", s.handleLens)
	mux.HandleFunc("POST /api/flip", s.handleFlip)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	mux.HandleFunc("POST /api/restart", s.handleRestart)

	// медіа-файли (знімки, записи) з теки даних
	mux.Handle("GET /media/", http.StripPrefix("/media/",
		http.FileServer(http.Dir(s.cfg.Capture.DataDir))))

	// статика React (якщо зібрана) з SPA-фолбеком на index.html
	if webDir != "" {
		mux.Handle("/", spaHandler(webDir))
	}

	return cors(mux)
}

// --- хендлери ---

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	type status struct {
		Model    string `json:"model"`
		Firmware string `json:"firmware"`
		Online   bool   `json:"online"`
		Motion   bool   `json:"motion_detection"`
	}
	st := status{}
	if di, err := s.cam.DeviceInfo(ctx); err == nil {
		st.Model = di.Model
		st.Firmware = di.FirmwareVersion
		st.Online = true
	}
	st.Motion, _ = s.cam.MotionDetectionEnabled(ctx)
	writeJSON(w, st)
}

func (s *Server) handleLiveSnapshot(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	jpg, err := s.cam.Snapshot(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(jpg)
}

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	path, err := s.cap.Capture(r.Context(), "manual")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]string{"path": path, "url": s.mediaURL(path)})
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 100)
	snaps, err := s.store.ListSnapshots(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type item struct {
		store.Snapshot
		URL string `json:"url"`
	}
	out := make([]item, 0, len(snaps))
	for _, sn := range snaps {
		out = append(out, item{Snapshot: sn, URL: s.mediaURL(sn.Path)})
	}
	writeJSON(w, out)
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 100)
	events, err := s.store.ListEvents(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type item struct {
		store.Event
		URL string `json:"url"`
	}
	out := make([]item, 0, len(events))
	for _, e := range events {
		url := ""
		if e.Path != "" {
			url = s.mediaURL(e.Path)
		}
		out = append(out, item{Event: e, URL: url})
	}
	writeJSON(w, out)
}

func (s *Server) handleClip(w http.ResponseWriter, r *http.Request) {
	path, err := s.rec.RecordClip(r.Context(), s.cfg.Capture.MotionClipSeconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.store.AddEvent(store.Event{Type: "recording", Path: path, CreatedAt: time.Now()})
	writeJSON(w, map[string]string{"path": path, "url": s.mediaURL(path)})
}

func (s *Server) handleLens(w http.ResponseWriter, r *http.Request) {
	var fn func(context.Context) error
	switch r.PathValue("action") {
	case "zoom_in":
		fn = s.cam.ZoomIn
	case "zoom_out":
		fn = s.cam.ZoomOut
	case "focus_near":
		fn = s.cam.FocusNear
	case "focus_far":
		fn = s.cam.FocusFar
	default:
		http.Error(w, "невідома дія", http.StatusBadRequest)
		return
	}
	if err := fn(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleFlip(w http.ResponseWriter, r *http.Request) {
	on, err := s.cam.ToggleFlip(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"flip": on})
}

// --- допоміжне ---

// mediaURL перетворює шлях на диску (dataDir/...) на URL /media/...
func (s *Server) mediaURL(path string) string {
	rel, err := filepath.Rel(s.cfg.Capture.DataDir, path)
	if err != nil {
		return ""
	}
	return "/media/" + filepath.ToSlash(rel)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func queryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler віддає статику, а на невідомі шляхи — index.html (SPA-роутинг).
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		if _, err := os.Stat(filepath.Join(dir, clean)); os.IsNotExist(err) && !strings.HasPrefix(clean, "/api") {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}

// Start запускає HTTP-сервер (блокує).
func (s *Server) Start(ctx context.Context, addr, webDir string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler(webDir)}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	log.Printf("api: слухаю %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
