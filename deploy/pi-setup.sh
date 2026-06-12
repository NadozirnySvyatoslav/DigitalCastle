#!/usr/bin/env bash
# DigitalCastle — налаштування на Raspberry Pi (запускати НА Pi через SSH).
#
# Робить:
#   1. ставить ffmpeg
#   2. перетворює Ethernet (eth0) на DHCP-сервер + NAT (WiFi — аплінк)
#   3. резервує камері фіксований IP по її MAC
#   4. ставить сервіс DigitalCastle як systemd-юніт (автозапуск)
#
# Очікує поряд (у тій самій теці): digitalcastle (бінарник arm64), web/,
# digitalcastle.service, config.yaml (або config.yaml.example), опційно .env
set -euo pipefail

# --- параметри (можна перевизначити змінними оточення) ---
ETH_IFACE="${ETH_IFACE:-eth0}"          # порт для камери
PI_ETH_IP="${PI_ETH_IP:-192.168.1.1/24}" # адреса Pi на цьому порту
CAM_MAC="${CAM_MAC:-d4:e8:53:81:e4:a1}"  # MAC камери
CAM_IP="${CAM_IP:-192.168.1.64}"         # фіксований IP для камери (резервація)
DEST=/opt/digitalcastle
HERE="$(cd "$(dirname "$0")" && pwd)"

echo "==> 1/4 Залежності (ffmpeg)..."
sudo apt-get update -qq
sudo apt-get install -y -qq ffmpeg

echo "==> 2/4 Мережа: $ETH_IFACE → DHCP-сервер + NAT ($PI_ETH_IP)..."
# NetworkManager 'shared' = статичний IP + вбудований DHCP (dnsmasq) + NAT у аплінк (WiFi)
sudo nmcli connection delete digitalcastle-cam 2>/dev/null || true
sudo nmcli connection add type ethernet ifname "$ETH_IFACE" con-name digitalcastle-cam \
  ipv4.method shared ipv4.addresses "$PI_ETH_IP" connection.autoconnect yes

echo "==> 3/4 DHCP-резервація камери ($CAM_MAC → $CAM_IP)..."
sudo mkdir -p /etc/NetworkManager/dnsmasq-shared.d
echo "dhcp-host=$CAM_MAC,$CAM_IP" \
  | sudo tee /etc/NetworkManager/dnsmasq-shared.d/digitalcastle-camera.conf >/dev/null
sudo nmcli connection up digitalcastle-cam

echo "==> 4/4 Сервіс DigitalCastle ($DEST)..."
sudo mkdir -p "$DEST"
sudo cp "$HERE/digitalcastle" "$DEST/"
sudo cp -r "$HERE/web" "$DEST/"
if [ -f "$HERE/config.yaml" ]; then
  sudo cp "$HERE/config.yaml" "$DEST/"
elif [ ! -f "$DEST/config.yaml" ]; then
  sudo cp "$HERE/config.yaml.example" "$DEST/config.yaml"
  echo "    ! Створено $DEST/config.yaml з прикладу — відредагуй під камеру."
fi
[ -f "$HERE/.env" ] && sudo cp "$HERE/.env" "$DEST/" || true
sudo chmod +x "$DEST/digitalcastle"
sudo chown -R "$USER:$USER" "$DEST"

sudo cp "$HERE/digitalcastle.service" /etc/systemd/system/digitalcastle.service
sudo sed -i "s/__USER__/$USER/" /etc/systemd/system/digitalcastle.service
sudo systemctl daemon-reload
sudo systemctl enable --now digitalcastle

echo
echo "✅ Готово."
echo "   Камера: переведи в DHCP-клієнт і підключи в порт $ETH_IFACE — отримає $CAM_IP."
echo "   Веб:    http://$(hostname -I | awk '{print $1}'):8080"
echo "   Логи:   journalctl -u digitalcastle -f"
echo "   Сервіс: sudo systemctl status digitalcastle"
