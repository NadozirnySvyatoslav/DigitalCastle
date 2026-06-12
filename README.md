# 🏰 DigitalCastle — локальний відеореєстратор для Hikvision

Go-бекенд + React-фронтенд для IP-камери Hikvision (DS-2CD1743G0-IZ): періодичні
знімки, запис відео на детектор руху, керування через Telegram-бот із розумним
LLM-пошуком, веб-інтерфейс із live-переглядом та налаштуваннями.

## Можливості

- 📸 Знімок кожні 5 хв → диск (тека за датою) + SQLite
- 🎥 Відеокліп при детекції руху (ffmpeg RTSP→mp4)
- 🤖 Telegram-бот: `/snap`, `/clip`, `/status`, `/help` + авто-пуш фото при русі
- 🔧 (в роботі) керування зумом/фокусом об'єктива та переворотом зображення
- 🖥️ (в роботі) веб-інтерфейс React: галерея, live-перегляд, контроли

## Вимоги

- Go 1.23+
- ffmpeg (запис кліпів)
- Доступ до камери в мережі (за замовч. 192.168.1.64)

## Налаштування

1. Скопіюй `config.yaml.example` → `config.yaml`, впиши IP, логін/пароль камери,
   токен Telegram-бота і свій `chat_id`.
2. `chat_id`: напиши боту `/start`, потім візьми `chat.id` з
   `https://api.telegram.org/bot<TOKEN>/getUpdates`.

## Запуск

```bash
go build -o nvr ./cmd/nvr

./nvr -selftest        # перевірити зв'язок з камерою
./nvr -capture-once    # зробити один знімок
./nvr -motion-test     # увімкнути детектор і слухати рух 60с
./nvr                  # демон: знімки + рух + Telegram-бот
```

## Структура

```
cmd/nvr/          точка входу, режими запуску
internal/
  config/         завантаження config.yaml
  camera/         ISAPI-клієнт (digest): deviceInfo, snapshot, motion, lens
  capture/        планувальник знімків
  recorder/       запис кліпів через ffmpeg
  store/          SQLite (метадані знімків і подій)
  bot/            Telegram-бот
  api/            (далі) REST API для фронтенду
data/             знімки, записи, БД (gitignored)
web/              (далі) React + Vite
```

## Камера

- Модель: DS-2CD1743G0-IZ (4MP, моторизований об'єктив 2.8–12 мм, ІЧ)
- Pan/tilt відсутній; керований лише зум/фокус + цифровий переворот
- RTSP: `…/Streaming/Channels/101` (4MP), `…/102` (SD)
