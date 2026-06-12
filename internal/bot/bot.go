package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"nvr/internal/camera"
	"nvr/internal/llm"
	"nvr/internal/recorder"
	"nvr/internal/store"
)

// Bot обробляє команди Telegram і надсилає сповіщення.
type Bot struct {
	api         *api
	chats       []int64        // усі дозволені чати (сповіщення йдуть в усі)
	allowed     map[int64]bool // швидка перевірка дозволу команд
	cam         camera.Camera
	rec         *recorder.Recorder
	store       *store.Store
	llm         *llm.Client // може бути nil, якщо LLM не налаштовано
	clipSeconds int
}

func New(token string, chats []int64, cam camera.Camera, rec *recorder.Recorder,
	st *store.Store, llmClient *llm.Client, clipSeconds int) *Bot {
	allowed := make(map[int64]bool, len(chats))
	for _, id := range chats {
		allowed[id] = true
	}
	return &Bot{
		api:         newAPI(token),
		chats:       chats,
		allowed:     allowed,
		cam:         cam,
		rec:         rec,
		store:       st,
		llm:         llmClient,
		clipSeconds: clipSeconds,
	}
}

// reply відповідає в конкретний чат (на команду).
func (b *Bot) reply(ctx context.Context, chatID int64, text string) error {
	return b.api.sendMessage(ctx, chatID, text)
}

func (b *Bot) replyPhoto(ctx context.Context, chatID int64, jpg []byte, caption string) error {
	return b.api.sendPhoto(ctx, chatID, jpg, caption)
}

// Broadcast надсилає текст у всі дозволені чати (сповіщення).
func (b *Bot) Broadcast(ctx context.Context, text string) {
	for _, id := range b.chats {
		b.api.sendMessage(ctx, id, text)
	}
}

// BroadcastPhoto надсилає знімок у всі дозволені чати.
func (b *Bot) BroadcastPhoto(ctx context.Context, jpg []byte, caption string) {
	for _, id := range b.chats {
		b.api.sendPhoto(ctx, id, jpg, caption)
	}
}

// BroadcastVideoFile читає файл і надсилає як відео в усі чати.
func (b *Bot) BroadcastVideoFile(ctx context.Context, path, caption string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, id := range b.chats {
		b.api.sendVideo(ctx, id, data, caption)
	}
	return nil
}

const helpText = `🎥 DigitalCastle — команди:
/snap — зробити знімок зараз
/clip — записати короткий відеокліп
/status — стан камери
/zoom_in /zoom_out — зум об'єктива
/focus_near /focus_far — фокус
/flip — перевернути зображення (дзеркало)
/find <запит> — 🔎 розумний пошук записів/знімків
/id — показати ID цього чату
/help — ця довідка

🔎 Просто напиши, що шукаєш, напр.:
«знайди записи за сьогодні вранці»
«покажи знімки за останню годину»`

// Run запускає цикл long-polling до скасування ctx.
func (b *Bot) Run(ctx context.Context) {
	log.Println("bot: запущено (long-polling)")
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		updates, err := b.api.getUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("bot: getUpdates: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1

			// натискання inline-кнопки
			if u.CallbackQuery != nil {
				cq := u.CallbackQuery
				var chatID int64
				if cq.Message != nil {
					chatID = cq.Message.Chat.ID
				}
				if !b.allowed[chatID] {
					continue
				}
				b.api.answerCallback(ctx, cq.ID)
				b.handleCallback(ctx, chatID, cq.Data)
				continue
			}

			if u.Message == nil {
				continue
			}
			// лише дозволені чати
			if !b.allowed[u.Message.Chat.ID] {
				log.Printf("bot: ігнор чужого чату %d (@%s)", u.Message.Chat.ID, u.Message.From.Username)
				continue
			}
			b.handle(ctx, u.Message.Chat.ID, strings.TrimSpace(u.Message.Text))
		}
	}
}

func (b *Bot) handle(ctx context.Context, chatID int64, text string) {
	// не команда (без "/") → природномовний пошук через LLM
	if text != "" && !strings.HasPrefix(text, "/") {
		b.handleSearch(ctx, chatID, text)
		return
	}

	cmd := strings.ToLower(strings.Fields(text+" ")[0])
	cmd = strings.TrimSuffix(cmd, "@mycamerahikvisionbot")
	args := strings.TrimSpace(strings.TrimPrefix(text, strings.Fields(text+" ")[0]))
	log.Printf("bot: команда %q з чату %d", cmd, chatID)

	switch cmd {
	case "/find", "/search", "/знайти":
		if args == "" {
			b.reply(ctx, chatID, "Напиши, що шукати. Напр.: /find записи за сьогодні вранці")
			return
		}
		b.handleSearch(ctx, chatID, args)

	case "/start", "/help":
		b.reply(ctx, chatID, helpText)

	case "/snap", "/photo":
		jpg, err := b.cam.Snapshot(ctx)
		if err != nil {
			b.reply(ctx, chatID, "❌ Помилка знімка: "+err.Error())
			return
		}
		b.replyPhoto(ctx, chatID, jpg, "📸 "+time.Now().Format("2006-01-02 15:04:05"))

	case "/clip":
		b.reply(ctx, chatID, fmt.Sprintf("🎬 Записую %dс...", b.clipSeconds))
		path, err := b.rec.RecordClip(ctx, b.clipSeconds)
		if err != nil {
			b.reply(ctx, chatID, "❌ Помилка запису: "+err.Error())
			return
		}
		if data, err := os.ReadFile(path); err == nil {
			b.api.sendVideo(ctx, chatID, data, "🎥 "+time.Now().Format("15:04:05"))
		} else {
			b.reply(ctx, chatID, "❌ Помилка надсилання відео: "+err.Error())
		}

	case "/status":
		b.handleStatus(ctx, chatID)

	case "/zoom_in", "/zoom_out", "/focus_near", "/focus_far":
		if !b.cam.Capabilities().Lens {
			b.reply(ctx, chatID, "🔧 Ця камера не підтримує керування об'єктивом.")
			return
		}
		switch cmd {
		case "/zoom_in":
			b.lens(ctx, chatID, b.cam.ZoomIn, "🔍 Зум +")
		case "/zoom_out":
			b.lens(ctx, chatID, b.cam.ZoomOut, "🔍 Зум −")
		case "/focus_near":
			b.lens(ctx, chatID, b.cam.FocusNear, "🎯 Фокус ближче")
		case "/focus_far":
			b.lens(ctx, chatID, b.cam.FocusFar, "🎯 Фокус далі")
		}
	case "/flip":
		if !b.cam.Capabilities().Flip {
			b.reply(ctx, chatID, "🔧 Ця камера не підтримує переворот зображення.")
			return
		}
		on, err := b.cam.ToggleFlip(ctx)
		if err != nil {
			b.reply(ctx, chatID, "❌ "+err.Error())
			return
		}
		b.reply(ctx, chatID, "🔄 Переворот зображення: "+onOff(on))

	case "/id":
		b.reply(ctx, chatID, fmt.Sprintf("ID цього чату: %d", chatID))

	default:
		b.reply(ctx, chatID, "Невідома команда. /help — список.")
	}
}

// handleSearch розбирає природномовний запит через LLM і пропонує кнопки до файлів.
func (b *Bot) handleSearch(ctx context.Context, chatID int64, query string) {
	if b.llm == nil {
		b.reply(ctx, chatID, "🔎 Розумний пошук не налаштовано (немає Azure-ключа в .env).")
		return
	}
	q, err := b.llm.ParseSearch(ctx, query, time.Now())
	if err != nil {
		b.reply(ctx, chatID, "❌ Не вдалось розібрати запит: "+err.Error())
		return
	}
	from, to := q.TimeRange()

	var buttons []InlineButton
	if q.Kind == "snapshot" {
		snaps, err := b.store.ListSnapshotsBetween(from, to, q.Limit)
		if err != nil {
			b.reply(ctx, chatID, "❌ Помилка пошуку: "+err.Error())
			return
		}
		for _, s := range snaps {
			buttons = append(buttons, InlineButton{
				Text: "📸 " + s.TakenAt.Format("02.01 15:04:05"),
				Data: "s:" + strconv.FormatInt(s.ID, 10),
			})
		}
	} else {
		recs, err := b.store.ListRecordingsBetween(from, to, q.Limit)
		if err != nil {
			b.reply(ctx, chatID, "❌ Помилка пошуку: "+err.Error())
			return
		}
		for _, e := range recs {
			buttons = append(buttons, InlineButton{
				Text: "🎥 " + e.CreatedAt.Format("02.01 15:04:05"),
				Data: "r:" + strconv.FormatInt(e.ID, 10),
			})
		}
	}

	if len(buttons) == 0 {
		b.reply(ctx, chatID, q.Reply+"\n\n🤷 Нічого не знайдено за цим періодом.")
		return
	}
	b.api.sendMessageWithButtons(ctx, chatID,
		fmt.Sprintf("%s\nЗнайдено %d. Обери, що надіслати:", q.Reply, len(buttons)), buttons)
}

// handleCallback надсилає вибраний знімок/запис при натисканні кнопки.
func (b *Bot) handleCallback(ctx context.Context, chatID int64, data string) {
	kind, idStr, ok := strings.Cut(data, ":")
	if !ok {
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return
	}
	switch kind {
	case "s":
		sn, err := b.store.SnapshotByID(id)
		if err != nil {
			b.reply(ctx, chatID, "❌ Знімок не знайдено в базі")
			return
		}
		jpg, err := os.ReadFile(sn.Path)
		if err != nil {
			b.reply(ctx, chatID, "❌ Файл знімка відсутній (можливо, прибрано автоочисткою)")
			return
		}
		b.replyPhoto(ctx, chatID, jpg, "📸 "+sn.TakenAt.Format("2006-01-02 15:04:05"))
	case "r":
		e, err := b.store.EventByID(id)
		if err != nil {
			b.reply(ctx, chatID, "❌ Запис не знайдено в базі")
			return
		}
		mp4, err := os.ReadFile(e.Path)
		if err != nil {
			b.reply(ctx, chatID, "❌ Файл запису відсутній (можливо, прибрано автоочисткою)")
			return
		}
		b.api.sendVideo(ctx, chatID, mp4, "🎥 "+e.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

// lens виконує рух об'єктива і надсилає оновлений знімок із результатом.
func (b *Bot) lens(ctx context.Context, chatID int64, action func(context.Context) error, label string) {
	if err := action(ctx); err != nil {
		b.reply(ctx, chatID, "❌ "+label+": "+err.Error())
		return
	}
	// дати об'єктиву/автофокусу стабілізуватись
	time.Sleep(1500 * time.Millisecond)
	jpg, err := b.cam.Snapshot(ctx)
	if err != nil {
		b.reply(ctx, chatID, label+" ✅")
		return
	}
	b.replyPhoto(ctx, chatID, jpg, label+" ✅")
}

func (b *Bot) handleStatus(ctx context.Context, chatID int64) {
	di, err := b.cam.DeviceInfo(ctx)
	if err != nil {
		b.reply(ctx, chatID, "❌ Камера недоступна: "+err.Error())
		return
	}
	motion, _ := b.cam.MotionDetectionEnabled(ctx)
	msg := fmt.Sprintf("📡 Стан камери:\nМодель: %s\nПрошивка: %s\nДетектор руху: %s",
		di.Model, di.FirmwareVersion, onOff(motion))
	b.reply(ctx, chatID, msg)
}

func onOff(b bool) string {
	if b {
		return "увімкнено ✅"
	}
	return "вимкнено ⛔"
}
