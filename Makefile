PI_HOST    := pi@pimusic.local
PI_DIR     := /home/pi/go/src/github.com/jaredwarren/rpi_music
SERVICE    := pplayer
BINARY     := pplayer
GOBIN      := $(shell go env GOBIN)

ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

GOFUMPT    := $(GOBIN)/gofumpt
GOVULNCHECK := $(GOBIN)/govulncheck

# ── Local ────────────────────────────────────────────────────────────────────

.PHONY: build run tools test test-race fmt lint vulncheck tidy-check check check-ci

build:
	go build -o $(BINARY) .

run:
	go run main.go 2>&1 | hl

tools:
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

test:
	go test -count=1 ./...

test-race:
	go test -race -count=1 ./...

fmt: $(GOFUMPT)
	$(GOFUMPT) -w .

lint:
	golangci-lint run ./...

vulncheck: $(GOVULNCHECK)
	$(GOVULNCHECK) ./...

tidy-check:
	go mod tidy
	git diff --exit-code go.mod go.sum

check: fmt lint test-race

check-ci: lint test-race tidy-check

$(GOFUMPT):
	go install mvdan.cc/gofumpt@latest

$(GOVULNCHECK):
	go install golang.org/x/vuln/cmd/govulncheck@latest

# ── Raspberry Pi ─────────────────────────────────────────────────────────────

.PHONY: pi-build pi-push pi-deploy pi-ensure-service pi-restart pi-logs pi-status pi-ssh

## Cross-compile for ARMv6 (Pi Zero / Pi 1)
pi-build:
	GOARM=6 GOARCH=arm GOOS=linux go build -o $(BINARY) .

## Push all runtime files to the Pi (binary + assets, not song/thumb data)
pi-push:
	ssh $(PI_HOST) "sudo install -d -o pi -g pi $(PI_DIR) $(PI_DIR)/config $(PI_DIR)/sounds $(PI_DIR)/public $(PI_DIR)/static $(PI_DIR)/templates"
	rsync -av --progress $(BINARY) $(PI_HOST):/tmp/$(BINARY)
	ssh $(PI_HOST) "sudo install -o pi -g pi -m 0755 /tmp/$(BINARY) $(PI_DIR)/$(BINARY) && rm -f /tmp/$(BINARY)"
	ssh $(PI_HOST) "rm -rf /tmp/rpi_music_deploy && mkdir -p /tmp/rpi_music_deploy"
	rsync -av --progress templates sounds public static localhost.crt localhost.key $(PI_HOST):/tmp/rpi_music_deploy/
	ssh $(PI_HOST) "sudo rm -rf $(PI_DIR)/templates $(PI_DIR)/sounds $(PI_DIR)/public $(PI_DIR)/static && sudo cp -R /tmp/rpi_music_deploy/templates /tmp/rpi_music_deploy/sounds /tmp/rpi_music_deploy/public /tmp/rpi_music_deploy/static $(PI_DIR)/ && sudo install -o pi -g pi -m 0644 /tmp/rpi_music_deploy/localhost.crt $(PI_DIR)/localhost.crt && sudo install -o pi -g pi -m 0644 /tmp/rpi_music_deploy/localhost.key $(PI_DIR)/localhost.key && sudo chown -R pi:pi $(PI_DIR)/templates $(PI_DIR)/sounds $(PI_DIR)/public $(PI_DIR)/static && rm -rf /tmp/rpi_music_deploy"
	@echo "Push complete"

## Build for Pi, push everything, then restart the service
pi-deploy: pi-build pi-push pi-ensure-service pi-restart

## Ensure the systemd service unit exists on the Pi
pi-ensure-service:
	@if ssh $(PI_HOST) "sudo systemctl cat $(SERVICE)" >/dev/null 2>&1; then \
		echo "Service unit exists"; \
	else \
		echo "Service unit missing; installing"; \
		$(MAKE) pi-install-service; \
	fi

## Restart the systemd service on the Pi
pi-restart:
	ssh $(PI_HOST) "sudo systemctl restart $(SERVICE)"
	@echo "Service restarted"

## Install (or update) the systemd service unit file
pi-install-service:
	rsync -av --progress player.service $(PI_HOST):/tmp/$(SERVICE).service
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
