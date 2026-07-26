#!/usr/bin/env sh
set -eu

APP_NAME="ha-ev-charging-bridge"
REPO="${REPO:-dphbfs/ha-ev-charging-bridge}"
VERSION="${VERSION:-${1:-latest}}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
SERVICE_NAME="${SERVICE_NAME:-ha-ev-charging-bridge}"

if [ "$(id -u)" -eq 0 ]; then
  INSTALL_USER="${SUDO_USER:-root}"
else
  INSTALL_USER="${USER:-$(id -un)}"
fi

USER_HOME="$(getent passwd "$INSTALL_USER" | cut -d: -f6)"
if [ -z "$USER_HOME" ]; then
  echo "could not determine home directory for $INSTALL_USER" >&2
  exit 1
fi
INSTALL_GROUP="$(id -gn "$INSTALL_USER")"

APP_HOME="$USER_HOME/.ha-ev-charging-bridge"
BINARY_PATH="$INSTALL_DIR/$APP_NAME"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

run_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

require_cmd uname
require_cmd curl
require_cmd tar
require_cmd sha256sum
require_cmd getent
if [ "$(id -u)" -ne 0 ]; then
  require_cmd sudo
fi

case "$(uname -m)" in
  x86_64 | amd64) ASSET_ARCH="linux-amd64" ;;
  i386 | i686) ASSET_ARCH="linux-386" ;;
  aarch64 | arm64) ASSET_ARCH="linux-arm64" ;;
  armv7l | armv7*) ASSET_ARCH="linux-armv7" ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

if [ "$VERSION" = "latest" ]; then
  BASE_URL="https://github.com/$REPO/releases/latest/download"
else
  BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
fi

ASSET="$APP_NAME-$ASSET_ARCH.tar.gz"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

curl -fsSL "$BASE_URL/$ASSET" -o "$TMP_DIR/$ASSET"
curl -fsSL "$BASE_URL/$ASSET.sha256" -o "$TMP_DIR/$ASSET.sha256"
(cd "$TMP_DIR" && sha256sum -c "$ASSET.sha256")
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"

run_root install -d -m 0755 "$INSTALL_DIR"
run_root install -m 0755 "$TMP_DIR/$APP_NAME-$ASSET_ARCH" "$BINARY_PATH"
run_root install -d -o "$INSTALL_USER" -g "$INSTALL_GROUP" -m 0755 "$APP_HOME"

if [ ! -f "$APP_HOME/config.yaml" ]; then
  run_root tee "$APP_HOME/config.yaml" >/dev/null <<EOF
home_assistant:
  url: http://replace-me.local:8123
  token: replace-me
  event_types:
    - state_changed

chargers: []

runtime:
  database_path: $APP_HOME/bridge.db
  log_file: $APP_HOME/app.log
EOF
  run_root chown "$INSTALL_USER:$INSTALL_GROUP" "$APP_HOME/config.yaml"
  echo "created placeholder config at $APP_HOME/config.yaml; edit it before starting the service"
fi

run_root tee "/etc/systemd/system/$SERVICE_NAME.service" >/dev/null <<EOF
[Unit]
Description=Home Assistant EV Charging Bridge
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$INSTALL_USER
Group=$INSTALL_GROUP
Environment=HOME=$USER_HOME
Environment=CONFIG_FILE=$APP_HOME/config.yaml
Environment=DATABASE_PATH=$APP_HOME/bridge.db
Environment=LOG_FILE=$APP_HOME/app.log
WorkingDirectory=$APP_HOME
ExecStart=$BINARY_PATH
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

run_root systemctl daemon-reload
run_root systemctl enable "$SERVICE_NAME"

echo "installed $APP_NAME to $BINARY_PATH"
echo "runtime directory: $APP_HOME"
echo "service installed: $SERVICE_NAME"
echo "start with: sudo systemctl start $SERVICE_NAME"
