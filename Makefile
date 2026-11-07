export PATH := $(CURDIR)/.tools/go/bin:$(PATH)
export GOCACHE := $(CURDIR)/.gocache
export GOPATH := $(CURDIR)/.tools/gopath
WHISPER_DIR := third_party/whisper.cpp
ROOT_DIR := $(shell pwd)
HOST_OS := $(shell uname -s)

export PKG_CONFIG_PATH := $(ROOT_DIR)/third_party/portaudio_static:$(PKG_CONFIG_PATH)
export C_INCLUDE_PATH := $(ROOT_DIR)/$(WHISPER_DIR)/include:$(ROOT_DIR)/$(WHISPER_DIR)/ggml/include
export LIBRARY_PATH := $(ROOT_DIR)/$(WHISPER_DIR)/build/src:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src/ggml-metal:$(ROOT_DIR)/$(WHISPER_DIR)/build/ggml/src/ggml-blas

export CGO_ENABLED := 1
export CGO_CFLAGS := -I$(ROOT_DIR)/third_party/portaudio/include
ifeq ($(HOST_OS),Darwin)
export MACOSX_DEPLOYMENT_TARGET := 14.2
export CGO_LDFLAGS := -L$(ROOT_DIR)/third_party/portaudio_static -framework Foundation -framework Metal -framework MetalKit -framework Accelerate -framework CoreAudio -framework AudioToolbox -framework AudioUnit -framework CoreFoundation -framework CoreServices
else ifeq ($(HOST_OS),Linux)
export CGO_LDFLAGS := -L$(ROOT_DIR)/third_party/portaudio_static -lasound -ldl -lpthread -lm
endif

.PHONY: setup build test test-ui race clean bundle package

setup:
	@bash scripts/setup.sh

build:
	go build -o tala ./cmd/tala

run: build
	./tala

test:
	go test ./...

test-ui:
	node --test internal/dashboard/tests/*.test.cjs

race:
	go test -race ./...

bundle:
	@test "$(HOST_OS)" = Darwin || { echo "App bundles require macOS. Use make package for the Linux CLI."; exit 1; }
	bash scripts/bundle.sh

package:
ifeq ($(HOST_OS),Linux)
	bash scripts/package-linux.sh
else
	bash scripts/package.sh
endif

clean:
	@bash scripts/clean.sh
