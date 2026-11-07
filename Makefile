export PATH := $(CURDIR)/.tools/go/bin:$(PATH)
export GOCACHE := $(CURDIR)/.gocache
export GOPATH := $(CURDIR)/.tools/gopath
WHISPER_DIR := third_party/whisper.cpp
ROOT_DIR := $(shell pwd)

export PKG_CONFIG_PATH := $(ROOT_DIR)/third_party/portaudio_static:$(shell brew --prefix portaudio 2>/dev/null)/lib/pkgconfig
export MACOSX_DEPLOYMENT_TARGET := 14.2
export C_INCLUDE_PATH := $(ROOT_DIR)/$(WHISPER_DIR)/include:$(ROOT_DIR)/$(WHISPER_DIR)/ggml/include
export LIBRARY_PATH := $(ROOT_DIR)/$(WHISPER_DIR)/build/src:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src/ggml-metal:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src/ggml-blas

export CGO_CFLAGS := -I$(ROOT_DIR)/third_party/portaudio/include -I$(shell brew --prefix portaudio 2>/dev/null || echo /opt/homebrew/opt/portaudio)/include
export CGO_LDFLAGS := -L$(ROOT_DIR)/third_party/portaudio_static -framework Foundation -framework Metal -framework MetalKit -framework Accelerate -framework CoreAudio -framework AudioToolbox -framework AudioUnit -framework CoreFoundation -framework CoreServices

.PHONY: setup build test race clean bundle package

setup:
	@bash scripts/setup.sh

build:
	go build -o kian ./cmd/kian

run: build
	./kian

test:
	go test ./...

race:
	go test -race ./...

bundle:
	bash scripts/bundle.sh

package:
	bash scripts/package.sh

clean:
	@bash scripts/clean.sh
