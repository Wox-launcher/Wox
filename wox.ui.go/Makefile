.PHONY: build clean embed-resources

UI_GO_DIR := ../wox.core/resource/ui/go
STANDALONE_DIR := build
WEBVIEW2_LOADER ?= ../wox.core/resource/ui/flutter/wox/WebView2Loader.dll

ifeq ($(OS),Windows_NT)
	UI_BINARY := $(STANDALONE_DIR)/wox-ui.exe
else
	UI_BINARY := $(STANDALONE_DIR)/wox-ui
endif

clean:
	rm -rf $(UI_GO_DIR)
	rm -rf $(STANDALONE_DIR)

embed-resources:
	rm -rf $(UI_GO_DIR)
	mkdir -p $(UI_GO_DIR)
ifeq ($(OS),Windows_NT)
	@test -f "$(WEBVIEW2_LOADER)" || { echo "WebView2Loader.dll was not found; set WEBVIEW2_LOADER to the WebView2 SDK loader path" >&2; exit 1; }
	cp "$(WEBVIEW2_LOADER)" $(UI_GO_DIR)/WebView2Loader.dll
endif
	touch $(UI_GO_DIR)/placeholder

build:
	rm -rf $(STANDALONE_DIR)
	mkdir -p $(STANDALONE_DIR)
	CGO_ENABLED=1 go build -ldflags "-s -w" -o $(UI_BINARY) ./cmd/wox-ui
ifeq ($(OS),Windows_NT)
	@test -f "$(WEBVIEW2_LOADER)" || { echo "WebView2Loader.dll was not found; set WEBVIEW2_LOADER to the WebView2 SDK loader path" >&2; exit 1; }
	cp "$(WEBVIEW2_LOADER)" $(STANDALONE_DIR)/WebView2Loader.dll
endif
	chmod +x $(UI_BINARY)
