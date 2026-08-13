# Makefile for wire-pod chipper
# Created by PicoClaw 🦞

.PHONY: all build clean test run

all: build

build:
	@echo "Building wire-pod chipper..."
	@go build -o chipper ./cmd/chipper

clean:
	@rm -f chipper
	@echo "Cleaned build artifacts"

test:
	@go test ./...

run: build
	@./start.sh

# Default target
default: build.PHONY: arm64 armv7

arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -o chipper.arm64 ./cmd/vosk

armv7:
	GOOS=linux GOARCH=arm GOARM=7 go build -o chipper.armv7 ./cmd/vosk