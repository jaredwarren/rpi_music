PI_HOST    := pi@pimusic.local
PI_DIR     := /home/pi/go/src/github.com/jaredwarren/rpi_music
SERVICE    := pplayer
BINARY     := pplayer

# ── Local ────────────────────────────────────────────────────────────────────

.PHONY: build run test fmt lint check

build:
	go build -o $(BINARY) .

run:
	go run main.go

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run ./...

check: fmt lint test

# ── Raspberry Pi ─────────────────────────────────────────────────────────────

.PHONY: pi-build pi-push pi-deploy pi-restart pi-logs pi-status pi-ssh

## Cross-compile for ARMv6 (Pi Zero / Pi 1)
pi-build:
	GOARM=6 GOARCH=arm GOOS=linux go build -o $(BINARY) .

## Push all runtime files to the Pi (binary + assets, not song/thumb data)
pi-push:
	ssh $(PI_HOST) "mkdir -p $(PI_DIR)/config $(PI_DIR)/sounds $(PI_DIR)/public $(PI_DIR)/static $(PI_DIR)/templates"
	scp $(BINARY)                  $(PI_HOST):$(PI_DIR)/$(BINARY)
	scp -r templates               $(PI_HOST):$(PI_DIR)/
	scp -r sounds                  $(PI_HOST):$(PI_DIR)/
	scp -r public                  $(PI_HOST):$(PI_DIR)/
	scp -r static                  $(PI_HOST):$(PI_DIR)/
	scp localhost.crt localhost.key $(PI_HOST):$(PI_DIR)/
	@echo "Push complete"

## Build for Pi, push everything, then restart the service
pi-deploy: pi-build pi-push pi-restart

## Restart the systemd service on the Pi
pi-restart:
	ssh $(PI_HOST) "sudo systemctl restart $(SERVICE)"
	@echo "Service restarted"

## Install (or update) the systemd service unit file
pi-install-service:
	scp player.service $(PI_HOST):/tmp/$(SERVICE).service
	ssh $(PI_HOST) "sudo mv /tmp/$(SERVICE).service /etc/systemd/system/$(SERVICE).service && sudo systemctl daemon-reload && sudo systemctl enable $(SERVICE)"
	@echo "Service installed and enabled"

## Tail the service log on the Pi
pi-logs:
	ssh $(PI_HOST) "sudo journalctl -u $(SERVICE) -f"

## Show service status on the Pi
pi-status:
	ssh $(PI_HOST) "sudo systemctl status $(SERVICE)"

## Open an SSH shell to the Pi
pi-ssh:
	ssh $(PI_HOST)
