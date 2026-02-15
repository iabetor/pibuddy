.PHONY: build build-user build-arm64 deploy clean test

BINARY   := pibuddy
CMD_DIR  := ./cmd/pibuddy
OUT_DIR  := ./bin

# Raspberry Pi SSH target (override with: make deploy PI=user@host)
PI       ?= pi@raspberrypi.local
PI_DIR   ?= /home/pi/pibuddy

build:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=1 go build -o $(OUT_DIR)/$(BINARY) $(CMD_DIR)
	@echo "Built $(OUT_DIR)/$(BINARY)"

build-user:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=1 go build -o $(OUT_DIR)/pibuddy-user ./cmd/user
	@echo "Built $(OUT_DIR)/pibuddy-user"

build-arm64:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc \
		go build -o $(OUT_DIR)/$(BINARY)-arm64 $(CMD_DIR)
	@echo "Built $(OUT_DIR)/$(BINARY)-arm64"

deploy: build-arm64
	ssh $(PI) "mkdir -p $(PI_DIR)/configs $(PI_DIR)/models"
	scp $(OUT_DIR)/$(BINARY)-arm64 $(PI):$(PI_DIR)/$(BINARY)
	scp configs/pibuddy.yaml $(PI):$(PI_DIR)/configs/
	scp scripts/pibuddy.service $(PI):/tmp/pibuddy.service
	ssh $(PI) "sudo mv /tmp/pibuddy.service /etc/systemd/system/ && sudo systemctl daemon-reload"
	@echo "Deployed to $(PI):$(PI_DIR)"

test:
	go test ./...

clean:
	rm -rf $(OUT_DIR)
