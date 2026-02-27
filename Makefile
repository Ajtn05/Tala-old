WHISPER_DIR := third_party/whisper.cpp
ROOT_DIR := $(shell pwd)

export MACOSX_DEPLOYMENT_TARGET := 13.0
export C_INCLUDE_PATH := $(ROOT_DIR)/$(WHISPER_DIR)/include:$(ROOT_DIR)/$(WHISPER_DIR)/ggml/include
export LIBRARY_PATH := $(ROOT_DIR)/$(WHISPER_DIR)/build/src:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src/ggml-metal:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src/ggml-blas

export CGO_CFLAGS := -I$(shell brew --prefix portaudio 2>/dev/null || echo /opt/homebrew/opt/portaudio)/include
export CGO_LDFLAGS := -L$(ROOT_DIR)/third_party/portaudio_static -framework Foundation -framework Metal -framework MetalKit -framework Accelerate -framework CoreAudio -framework AudioToolbox -framework AudioUnit -framework CoreFoundation -framework CoreServices

.PHONY: setup build test clean

setup:
	@bash scripts/setup.sh

build:
	go build -o kian ./cmd/kian

run: build
	./kian

test:
	go test ./...

clean:
	rm -f kian
