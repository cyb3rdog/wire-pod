# Makefile for wire-pod chipper

.PHONY: all build clean test run arm64 armv7

all: build

# Matches the default in start.sh/setup.sh/dockerfile.
build:
	@echo "Building wire-pod chipper (vosk)..."
	@go build -tags nolibopusfile -o chipper ./cmd/vosk

clean:
	@rm -f chipper chipper.arm64 chipper.armv7
	@echo "Cleaned build artifacts"

# Excludes the STT backends that need their own native libraries
# (vosk/coqui/whisper.cpp) installed to even compile -- same filter the
# CI verify job uses. `make build` above still builds the real vosk
# binary if those libs are present on your system.
test:
	@go test $$(go list ./... | grep -v -E '/(cmd/coqui|cmd/vosk|cmd/experimental/whisper\.cpp|stt/coqui|stt/vosk|stt/whisper\.cpp)$$')

run: build
	@./start.sh

arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -tags nolibopusfile -o chipper.arm64 ./cmd/vosk

armv7:
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 go build -tags nolibopusfile -o chipper.armv7 ./cmd/vosk
