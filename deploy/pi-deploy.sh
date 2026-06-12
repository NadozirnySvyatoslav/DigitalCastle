#!/usr/bin/env bash
# Деплой DigitalCastle на Raspberry Pi одним рухом (запускати на НОУТБУЦІ).
#
# Що робить: крос-збирає arm64-бінарник, збирає фронтенд, пакує разом із
# config.yaml/.env та скриптами, копіює на Pi по SSH і запускає pi-setup.sh.
#
# Використання:
#   ./deploy/pi-deploy.sh <ssh-ціль>
#   напр.  ./deploy/pi-deploy.sh pi@digitalcastle.local
set -euo pipefail

TARGET="${1:?Вкажи SSH-ціль, напр.: ./deploy/pi-deploy.sh pi@digitalcastle.local}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> Збірка фронтенду..."
( cd web && npm run build )

echo "==> Крос-збірка бекенду (linux/arm64)..."
BUNDLE="$(mktemp -d)"
GOOS=linux GOARCH=arm64 go build -o "$BUNDLE/digitalcastle" ./cmd/nvr

echo "==> Пакування бандла..."
mkdir -p "$BUNDLE/web"
cp -r web/dist "$BUNDLE/web/dist"
cp config.yaml.example "$BUNDLE/"
[ -f config.yaml ] && cp config.yaml "$BUNDLE/"        # реальний конфіг (із секретами) — по SSH
[ -f .env ] && cp .env "$BUNDLE/"                      # Azure-ключі — по SSH
cp deploy/digitalcastle.service deploy/pi-setup.sh "$BUNDLE/"
chmod +x "$BUNDLE/pi-setup.sh"

echo "==> Копіювання на $TARGET..."
ssh "$TARGET" 'rm -rf ~/digitalcastle-bundle && mkdir -p ~/digitalcastle-bundle'
scp -q -r "$BUNDLE"/* "$TARGET":~/digitalcastle-bundle/

echo "==> Встановлення на Pi (потрібен sudo на Pi)..."
ssh -t "$TARGET" 'bash ~/digitalcastle-bundle/pi-setup.sh'

rm -rf "$BUNDLE"
echo "==> Деплой завершено."
