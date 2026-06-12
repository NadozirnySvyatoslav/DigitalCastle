# 🍓 Розгортання DigitalCastle на Raspberry Pi 5

Pi стає автономним пристроєм відеоспостереження:

```
Інтернет ──WiFi (wlan0)──▶ [ Raspberry Pi 5 ] ──Ethernet (eth0)──▶ Камера
                            • DHCP-сервер + NAT на eth0
                            • Pi = 192.168.1.1, камера = 192.168.1.64 (резервація по MAC)
                            • сервіс DigitalCastle (systemd, автозапуск)
```

- **WiFi** — аплінк (Telegram, Azure, NTP, оновлення).
- **Ethernet** — приватна мережа камери; Pi роздає IP по DHCP і NAT-ить камеру в інтернет.
- Камера — **DHCP-клієнт**, але завжди отримує `192.168.1.64` (резервація за MAC).

---

## Крок 1. Прошити флешку (Raspberry Pi Imager)

```bash
rpi-imager
```
1. **OS:** Raspberry Pi OS **Lite (64-bit)** (без робочого столу — це сервер).
2. **Storage:** твоя USB-флешка (⚠️ усе на ній зітреться).
3. **⚙️ (Edit settings):**
   - **Hostname:** `digitalcastle`
   - **Enable SSH** → *Allow public-key authentication only* → встав свій публічний ключ
     (`~/.ssh/id_ed25519.pub`).
   - **Set username and password:** напр. `svyat` (+ пароль — знадобиться для sudo).
   - **Configure wireless LAN:** SSID + пароль твого WiFi, країна.
   - **Locale:** часовий пояс.
4. **Write.**

## Крок 2. Завантажити Pi і зайти по SSH

Встав флешку в Pi, увімкни. За ~1 хв:
```bash
ssh svyat@digitalcastle.local       # без пароля, по ключу
```
> Якщо `.local` не резолвиться — знайди IP Pi у списку клієнтів роутера і заходь по ньому.

## Крок 3. Розгорнути сервіс

З **ноутбука** (з кореня проєкту), одним рухом:
```bash
./deploy/pi-deploy.sh svyat@digitalcastle.local
```
Скрипт крос-збере arm64-бінарник, збере фронтенд, скопіює все на Pi (разом із твоїм
`config.yaml` і `.env`) і запустить `pi-setup.sh`, який:
- поставить `ffmpeg`;
- зробить `eth0` DHCP-сервером + NAT;
- зарезервує камері `192.168.1.64`;
- поставить сервіс у systemd (автозапуск, рестарт при збої).

> На кроці встановлення Pi може спитати пароль для `sudo` — введи той, що задав у rpi-imager.

## Крок 4. Підключити камеру

1. У веб-інтерфейсі камери (або ISAPI) переведи мережу в **DHCP-клієнт**.
2. Підключи камеру кабелем у **Ethernet-порт Pi**.
3. Камера отримає `192.168.1.64`. Перевір:
   ```bash
   ssh svyat@digitalcastle.local
   journalctl -u digitalcastle -f         # логи сервісу
   ```
4. Відкрий веб-інтерфейс: **http://digitalcastle.local:8080**

---

## Корисні команди (на Pi)

```bash
sudo systemctl status digitalcastle      # стан
sudo systemctl restart digitalcastle     # рестарт
journalctl -u digitalcastle -f           # логи
nmcli connection show digitalcastle-cam  # мережа камери
cat /var/lib/NetworkManager/dnsmasq-*    # видані DHCP-адреси (lease)
```

## Оновлення сервісу

Просто перезапусти деплой — він перезбере й перельє новий бінарник:
```bash
./deploy/pi-deploy.sh svyat@digitalcastle.local
```

## Параметри (за потреби)

`pi-setup.sh` приймає змінні оточення:

| Змінна | Типово | Що це |
|---|---|---|
| `ETH_IFACE` | `eth0` | порт для камери |
| `PI_ETH_IP` | `192.168.1.1/24` | адреса Pi на цьому порту |
| `CAM_MAC` | `d4:e8:53:81:e4:a1` | MAC камери (для резервації) |
| `CAM_IP` | `192.168.1.64` | фіксований IP камери |

> ⚠️ Для запису 24/7 краще USB-**SSD**, а не флешка — флешки швидко зношуються від запису.
