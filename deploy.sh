#!/bin/bash
set -e

SERVICE_NAME="llm-bridge"
BINARY_PATH="/usr/local/bin/$SERVICE_NAME"
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"

echo "Building $SERVICE_NAME..."
go build -o "$SERVICE_NAME" ./cmd/llm-bridge

echo "Installing binary..."
sudo mv "$SERVICE_NAME" "$BINARY_PATH"
sudo chmod +x "$BINARY_PATH"

if [ ! -f "$SERVICE_FILE" ]; then
    echo "Creating systemd service..."
    sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=LLM Bridge Service
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=$BINARY_PATH
Restart=on-failure
RestartSec=5
Environment=LLMBRIDGE_LISTEN_ADDR=:8160

[Install]
WantedBy=multi-user.target
EOF
    sudo systemctl daemon-reload
    sudo systemctl enable "$SERVICE_NAME"
fi

echo "Restarting service..."
sudo systemctl restart "$SERVICE_NAME"
sudo systemctl status "$SERVICE_NAME" --no-pager

echo "Done. Service running on port 8160"
