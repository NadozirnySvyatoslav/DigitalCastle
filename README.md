<div align="center">

# 🏰 DigitalCastle

**Самодостатній відеореєстратор (NVR) для IP-камер — Go + React, з Telegram-ботом і розумним LLM-пошуком.**

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![ffmpeg](https://img.shields.io/badge/ffmpeg-RTSP-007808?logo=ffmpeg&logoColor=white)](https://ffmpeg.org)
[![SQLite](https://img.shields.io/badge/SQLite-pure--Go-003B57?logo=sqlite&logoColor=white)](https://modernc.org/sqlite)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Raspberry%20Pi-555)](#-розгортання-на-raspberry-pi)

</div>

DigitalCastle перетворює звичайний комп'ютер або Raspberry Pi на повноцінний
відеореєстратор для будь-якої IP-камери: періодичні знімки, запис відео за
детектором руху, веб-інтерфейс і керування через Telegram з пошуком записів
природною мовою.

> Камера робить усю важку роботу (кодує H.264, детектує рух) — сервіс лише зберігає
> готовий потік і віддає інтерфейс. Тому він легкий навіть для Raspberry Pi.

---

## 📸 Інтерфейс

| Live-перегляд | Налаштування |
|:---:|:---:|
| ![Live](docs/screenshots/live.png) | ![Налаштування](docs/screenshots/settings.png) |
| Живий кадр, керування об'єктивом, знімок/кліп | Усі параметри редагуються з браузера |

| Галерея знімків | Журнал подій |
|:---:|:---:|
| ![Галерея](docs/screenshots/snapshots.png) | ![Події](docs/screenshots/events.png) |
| Знімки за датою з підписами | Події руху та відеозаписи з плеєром |

---

## ✨ Можливості

- 📷 **Будь-яка камера** — універсальний RTSP-драйвер працює з будь-якою IP-камерою;
  Hikvision має окремий драйвер з апаратними функціями (див. [Драйвери](#-драйвери-камер)).
- 🖼️ **Знімки за розкладом** — JPEG кожні N хвилин у теку за датою + метадані в SQLite.
- 🎥 **Запис за рухом** — відеокліпи з RTSP при спрацюванні детектора (ffmpeg, без перекодування).
- 🏃 **Детектор руху** — апаратний (Hikvision) або програмний (аналіз сцен ffmpeg) для решти камер.
- 🤖 **Telegram-бот** — знімок, кліп, статус, керування зумом/фокусом; кілька дозволених чатів/груп.
- 🔎 **Розумний LLM-пошук** — «знайди записи за вчора з 16 до 18» → кнопки на потрібні файли (Azure OpenAI).
- 🖥️ **Веб-інтерфейс** — live-перегляд, галерея, журнал подій, повний редактор налаштувань.
- 🧹 **Автоочистка** — видалення старих файлів за віком і сумарним розміром.
- ⏰ **Синхронізація часу** — тримає годинник камери актуальним (для камер без NTP).
- 🔐 **Безпека за замовчуванням** — секрети у `config.yaml`/`.env` (поза git), доступ боту лише дозволеним чатам.

---

## 🏗️ Архітектура

<div align="center">

![Архітектура DigitalCastle](docs/screenshots/architecture.png)

</div>

> Діаграму згенеровано з [`docs/architecture.puml`](docs/architecture.puml) (PlantUML).

---

## 📷 Драйвери камер

Камера абстрагована інтерфейсом `camera.Camera`; драйвер обирається у конфізі.
Кожен повідомляє свої `Capabilities`, і бот/веб-інтерфейс **автоматично ховають
недоступні функції** (напр. кнопки зуму для camera без об'єктива).

| Драйвер | Камери | Знімки | Детектор руху | Зум/фокус | Flip | Синх. часу |
|---|---|:---:|:---:|:---:|:---:|:---:|
| `generic` | **будь-яка RTSP** | ffmpeg | програмний | — | — | — |
| `hikvision` | Hikvision (ISAPI) | ISAPI | апаратний | ✅ | ✅ | ✅ |

Для `generic` достатньо `rtsp_main`; чутливість програмного детектора —
`camera.motion_threshold` (0..1, типово 0.05).

---

## 🚀 Швидкий старт

### Вимоги
- **Go 1.23+**, **ffmpeg**, **Node 18+** (лише для збірки фронтенду)

### Збірка та запуск
```bash
# 1. фронтенд
cd web && npm install && npm run build && cd ..

# 2. бекенд
go build -o digitalcastle ./cmd/nvr

# 3. конфіг
cp config.yaml.example config.yaml   # відредагуй під свою камеру

# 4. запуск
./digitalcastle
```
Відкрий **http://localhost:8080**.

### Режими CLI
```bash
./digitalcastle                 # демон (за замовчуванням)
./digitalcastle -selftest       # перевірити зв'язок з камерою
./digitalcastle -capture-once   # зробити один знімок
./digitalcastle -motion-test    # перевірити детектор руху (60с)
./digitalcastle -cleanup-once   # ручне прибирання старих файлів
./digitalcastle -search-test "записи за вчора"   # перевірити LLM-розбір
```

---

## ⚙️ Конфігурація

`config.yaml` (повний приклад — у `config.yaml.example`):

```yaml
camera:
  type: "hikvision"        # або "generic" для будь-якої RTSP-камери
  host: "192.168.1.64"     # лише для hikvision
  username: "admin"
  password: "••••••"
  rtsp_main: "rtsp://admin:••••@192.168.1.64:554/Streaming/Channels/101"
  rtsp_sub:  "rtsp://admin:••••@192.168.1.64:554/Streaming/Channels/102"
  motion_threshold: 0.05   # generic: чутливість детектора (0..1)

telegram:
  token: "<токен від @BotFather>"
  chat_id: 0               # /id у боті покаже ваш ID
  allowed_chats: []        # додаткові чати/групи, напр. [-1001234567890]
  notify_on_motion: true

capture:
  snapshot_interval: "5m"
  data_dir: "./data"
  motion_clip_seconds: 15

retention:
  max_days: 14             # 0 = без ліміту за віком
  max_size_gb: 5.0         # 0 = без ліміту за розміром
  interval: "1h"

server:
  addr: ":8080"
```

**Azure OpenAI** (для LLM-пошуку) — у `.env`:
```
AZURE_API_KEY=...
AZURE_ENDPOINT=https://<ресурс>.cognitiveservices.azure.com/...
# опційно: AZURE_CHAT_DEPLOYMENT (типово gpt-4o-mini), AZURE_API_VERSION
```

> `config.yaml` та `.env` у `.gitignore` — секрети не потрапляють у репозиторій.

---

## 🤖 Telegram-бот

| Команда | Дія |
|---|---|
| `/snap` | знімок зараз |
| `/clip` | короткий відеокліп |
| `/status` | стан камери та детектора |
| `/zoom_in` `/zoom_out` | зум об'єктива (якщо підтримується) |
| `/focus_near` `/focus_far` | фокус |
| `/flip` | переворот зображення |
| `/find <запит>` | 🔎 розумний пошук записів/знімків |
| `/id` | показати ID чату |

**Розумний пошук:** у приватному чаті можна просто писати природною мовою
(«покажи знімки за останню годину»); у групах — через `/find`. Бот розбирає запит
через LLM і повертає inline-кнопки на до 10 відео/знімків.

---

## 🔌 REST API

| Метод | Шлях | Опис |
|---|---|---|
| `GET` | `/api/status` | модель, стан, capabilities |
| `GET` | `/api/snapshot/live` | свіжий кадр (JPEG) |
| `POST` | `/api/snapshot` | зробити й зберегти знімок |
| `GET` | `/api/snapshots?limit=N` | список знімків |
| `GET` | `/api/events?limit=N` | список подій |
| `POST` | `/api/clip` | записати кліп |
| `POST` | `/api/lens/{action}` | зум/фокус (`zoom_in`…) |
| `POST` | `/api/flip` | переворот зображення |
| `GET` `PUT` | `/api/config` | читання/збереження налаштувань |
| `POST` | `/api/restart` | застосувати налаштування (перезапуск) |
| `GET` | `/media/...` | знімки та відеозаписи |

---

## 🍓 Розгортання на Raspberry Pi

Pi 5 стає автономним пристроєм: **WiFi** — аплінк, **Ethernet** — приватна мережа
камери (Pi роздає DHCP + NAT), сервіс — у systemd з автозапуском.

```bash
# на ноутбуці, після прошивання Pi через rpi-imager (WiFi + SSH-ключ):
./deploy/pi-deploy.sh svyat@digitalcastle.local
```

Скрипт крос-збере arm64-бінарник (pure-Go, без CGO), збере фронтенд, скопіює все
на Pi і налаштує мережу + systemd. Повний покроковий гайд — у **[`deploy/README.md`](deploy/README.md)**.

> Для запису 24/7 краще USB-SSD замість SD-картки (флешки зношуються від запису).

---

## 🗺️ Плани

- [ ] ONVIF-драйвер (PTZ/події для не-Hikvision камер)
- [ ] HLS-стрім live-перегляду замість опитування кадрів
- [ ] Кілька камер
- [ ] Автентифікація у веб-інтерфейсі

---

## 🛠️ Технології

Go · React · TypeScript · Vite · ffmpeg · SQLite (modernc, pure-Go) ·
Azure OpenAI (gpt-4o-mini) · Telegram Bot API · Hikvision ISAPI

---

<div align="center">
<sub>Тестова камера: Hikvision DS-2CD1743G0-IZ (4MP, об'єктив 2.8–12 мм).</sub>
</div>
