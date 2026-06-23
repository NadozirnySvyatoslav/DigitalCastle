package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"nvr/internal/api"
	"nvr/internal/bot"
	"nvr/internal/camera"
	"nvr/internal/capture"
	"nvr/internal/cleanup"
	"nvr/internal/config"
	"nvr/internal/llm"
	"nvr/internal/recorder"
	"nvr/internal/store"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "шлях до конфігу")
	selftest := flag.Bool("selftest", false, "перевірити зв'язок з камерою і вийти")
	captureOnce := flag.Bool("capture-once", false, "зробити один знімок через сховище і вийти")
	motionTest := flag.Bool("motion-test", false, "увімкнути детектор руху і слухати події 60с")
	cleanupOnce := flag.Bool("cleanup-once", false, "виконати одне прибирання старих файлів і вийти")
	searchTest := flag.String("search-test", "", "перевірити LLM-розбір запиту і вийти")
	flag.Parse()

	// змінні оточення з .env (Azure-креди для LLM-пошуку)
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("увага: .env: %v", err)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("конфіг: %v", err)
	}

	cam, err := camera.New(camera.Options{
		Driver:          cfg.Camera.Type,
		Host:            cfg.Camera.Host,
		Username:        cfg.Camera.Username,
		Password:        cfg.Camera.Password,
		RTSPMain:        cfg.Camera.RTSPMain,
		RTSPSub:         cfg.Camera.RTSPSub,
		MotionThreshold: cfg.Camera.MotionThreshold,
	})
	if err != nil {
		log.Fatalf("камера: %v", err)
	}
	log.Printf("камера: драйвер %q", cam.Capabilities().Driver)
	configPath := *cfgPath

	if *selftest {
		runSelfTest(cam, cfg.Capture.DataDir)
		return
	}

	if *motionTest {
		runMotionTest(cam)
		return
	}

	if *searchTest != "" {
		runSearchTest(*searchTest)
		return
	}

	// сховище
	if err := os.MkdirAll(cfg.Capture.DataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	st, err := store.Open(filepath.Join(cfg.Capture.DataDir, "nvr.db"))
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	cap := capture.New(cam, st, cfg.Capture.DataDir, cfg.Capture.SnapshotInterval)

	if *cleanupOnce {
		cleanup.New(cfg, st).RunOnce()
		fmt.Println("Прибирання виконано.")
		return
	}

	if *captureOnce {
		path, err := cap.Capture(context.Background(), "manual")
		if err != nil {
			log.Fatalf("capture: %v", err)
		}
		fmt.Printf("Знімок: %s\n", path)
		snaps, _ := st.ListSnapshots(5, 0)
		fmt.Printf("У БД записів: %d (останній id=%d)\n", len(snaps), snaps[0].ID)
		return
	}

	// === основний режим: демон ===
	runDaemon(cfg, configPath, cam, st, cap)
}

func runDaemon(cfg *config.Config, configPath string, cam camera.Camera, st *store.Store, cap *capture.Service) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rec := recorder.New(cfg.Camera.RTSPMain, cfg.Capture.DataDir)

	// LLM-клієнт для розумного пошуку (необов'язковий)
	llmClient, err := llm.NewFromEnv()
	if err != nil {
		log.Printf("daemon: LLM-пошук вимкнено (%v)", err)
		llmClient = nil
	} else {
		log.Println("daemon: LLM-пошук увімкнено (Azure)")
	}

	// Telegram-бот (якщо налаштований)
	var tg *bot.Bot
	if chats := cfg.Telegram.Chats(); cfg.Telegram.Token != "" && len(chats) > 0 {
		tg = bot.New(cfg.Telegram.Token, chats, cam, rec, st, llmClient, cfg.Capture.MotionClipSeconds)
		go tg.Run(ctx)
		go tg.Broadcast(ctx, "🟢 NVR запущено. /help — команди")
		log.Printf("daemon: Telegram-бот для %d чат(ів)", len(chats))
	} else {
		log.Println("daemon: Telegram не налаштовано (немає token або chat_id)")
	}

	caps := cam.Capabilities()

	// Переконуємось, що детектор руху увімкнено (де це підтримується)
	if caps.MotionConfig {
		if err := cam.SetMotionDetection(ctx, true); err != nil {
			log.Printf("daemon: не вдалося увімкнути детектор руху: %v", err)
		}
	}

	// Синхронізація годинника камери — лише для камер, що це підтримують
	if caps.TimeSync {
		go syncCameraTime(ctx, cam)
	}

	// Планувальник знімків
	go cap.Run(ctx)

	// Детектор руху з дебаунсом
	go watchMotionWithDebounce(ctx, cfg, cam, st, rec, tg)

	// Прибирання старих файлів (за віком і розміром)
	go cleanup.New(cfg, st).Run(ctx)

	// REST API + статика React
	apiSrv := api.New(cfg, configPath, cam, st, cap, rec)
	webDir := "web/dist"
	if _, err := os.Stat(webDir); err != nil {
		webDir = "" // фронтенд ще не зібрано — лише API
	}
	go func() {
		if err := apiSrv.Start(ctx, cfg.Server.Addr, webDir); err != nil {
			log.Printf("api: %v", err)
		}
	}()

	log.Printf("daemon: працює. Знімки кожні %s, детектор руху активний, API на %s. Ctrl+C — зупинка.",
		cfg.Capture.SnapshotInterval, cfg.Server.Addr)
	<-ctx.Done()
	log.Println("daemon: зупинка...")
	time.Sleep(500 * time.Millisecond)
}

// syncCameraTime тримає годинник камери в актуальному стані (без NTP).
func syncCameraTime(ctx context.Context, cam camera.Camera) {
	sync := func() {
		if err := cam.SyncTime(ctx, time.Now()); err != nil {
			log.Printf("time: синхронізація: %v", err)
		} else {
			log.Printf("time: годинник камери синхронізовано")
		}
	}
	sync()
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sync()
		}
	}
}

// watchMotionWithDebounce реагує на рух не частіше, ніж раз на cooldown:
// зберігає знімок-подію, записує кліп і шле сповіщення в Telegram.
func watchMotionWithDebounce(ctx context.Context, cfg *config.Config, cam camera.Camera,
	st *store.Store, rec *recorder.Recorder, tg *bot.Bot) {

	const cooldown = 30 * time.Second
	var last time.Time

	cam.WatchMotion(ctx, func(e camera.MotionEvent) {
		if time.Since(last) < cooldown {
			return // дебаунс: подавляємо потік повторних подій
		}
		last = time.Now()
		log.Printf("motion: рух о %s — реагую", e.At.Format("15:04:05"))

		st.AddEvent(store.Event{Type: "motion", Note: e.Description, CreatedAt: e.At})

		// знімок + пуш у Telegram (усі дозволені чати)
		if jpg, err := cam.Snapshot(ctx); err == nil {
			if tg != nil && cfg.Telegram.NotifyOnMotion {
				tg.BroadcastPhoto(ctx, jpg, "🏃 Рух о "+e.At.Format("15:04:05"))
			}
		}

		// відеокліп
		go func(at time.Time) {
			path, err := rec.RecordClip(ctx, cfg.Capture.MotionClipSeconds)
			if err != nil {
				log.Printf("motion: запис кліпу: %v", err)
				return
			}
			st.AddEvent(store.Event{Type: "recording", Path: path, CreatedAt: at})
			log.Printf("motion: кліп збережено %s", path)
		}(e.At)
	})
}

func runSearchTest(query string) {
	c, err := llm.NewFromEnv()
	if err != nil {
		log.Fatalf("LLM: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	q, err := c.ParseSearch(ctx, query, time.Now())
	if err != nil {
		log.Fatalf("розбір: %v", err)
	}
	fmt.Printf("Запит:  %q\n", query)
	fmt.Printf("Kind:   %s\n", q.Kind)
	fmt.Printf("From:   %s\n", q.From)
	fmt.Printf("To:     %s\n", q.To)
	fmt.Printf("Limit:  %d\n", q.Limit)
	fmt.Printf("Reply:  %s\n", q.Reply)
}

func runMotionTest(cam camera.Camera) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	on, err := cam.MotionDetectionEnabled(ctx)
	if err != nil {
		log.Fatalf("стан детектора: %v", err)
	}
	fmt.Printf("Детектор руху був: %v\n", on)
	if !on {
		if err := cam.SetMotionDetection(ctx, true); err != nil {
			log.Fatalf("увімкнення детектора: %v", err)
		}
		fmt.Println("Детектор руху УВІМКНЕНО ✅")
	}

	fmt.Println("Слухаю події руху 60с — поворушись перед камерою...")
	cam.WatchMotion(ctx, func(e camera.MotionEvent) {
		fmt.Printf("  🏃 РУХ! %s %s — %s\n", e.At.Format("15:04:05"), e.Type, e.Description)
	})
	fmt.Println("Готово.")
}

func runSelfTest(cam camera.Camera, dataDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fmt.Println("== Перевірка зв'язку з камерою ==")
	di, err := cam.DeviceInfo(ctx)
	if err != nil {
		log.Fatalf("deviceInfo: %v", err)
	}
	fmt.Printf("  Модель:    %s\n", di.Model)
	fmt.Printf("  Прошивка:  %s\n", di.FirmwareVersion)

	fmt.Println("== Тестовий знімок ==")
	jpg, err := cam.Snapshot(ctx)
	if err != nil {
		log.Fatalf("snapshot: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "snapshots"), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	out := filepath.Join(dataDir, "snapshots", "selftest.jpg")
	if err := os.WriteFile(out, jpg, 0o644); err != nil {
		log.Fatalf("збереження: %v", err)
	}
	fmt.Printf("  Знімок збережено: %s (%d КБ)\n", out, len(jpg)/1024)
	fmt.Println("OK ✅")
}
