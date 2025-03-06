.PHONY: clean build help

# Determine the current platform
ifeq ($(OS),Windows_NT)
    FLUTTER_TARGET := windows
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Linux)
        FLUTTER_TARGET := linux
    endif
    ifeq ($(UNAME_S),Darwin)
        FLUTTER_TARGET := macos
    endif
endif

UI_FLUTTER_DIR := ../../wox.core/resource/ui/flutter

help:
	@echo "Available commands:"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make build      - Build Flutter UI for current platform ($(FLUTTER_TARGET))"

clean:
	flutter clean
	rm -rf $(UI_FLUTTER_DIR)

build: clean
	flutter build --release $(FLUTTER_TARGET)
	mkdir -p $(UI_FLUTTER_DIR)
ifeq ($(FLUTTER_TARGET),windows)
	cp -r build/$(FLUTTER_TARGET)/x64/runner/Release $(UI_FLUTTER_DIR)/wox
endif
ifeq ($(FLUTTER_TARGET),linux)
	cp -r build/$(FLUTTER_TARGET)/x64/release/bundle $(UI_FLUTTER_DIR)/wox
endif
ifeq ($(FLUTTER_TARGET),macos)
	cp -r build/$(FLUTTER_TARGET)/Build/Products/Release/wox.app $(UI_FLUTTER_DIR)
	chmod +x $(UI_FLUTTER_DIR)/wox.app/Contents/MacOS/wox
endif 