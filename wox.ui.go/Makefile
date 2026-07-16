.PHONY: clean embed-resources webview2-loader

UI_GO_DIR := ../wox.core/resource/ui/go
WEBVIEW2_VERSION ?= 1.0.1210.39
WEBVIEW2_PACKAGE_DIR := .deps/Microsoft.Web.WebView2
WEBVIEW2_LOADER ?= $(WEBVIEW2_PACKAGE_DIR)/build/native/x64/WebView2Loader.dll

ifeq ($(OS),Windows_NT)
WEBVIEW2_DEPENDENCY := webview2-loader

webview2-loader:
	@if [ ! -f "$(WEBVIEW2_LOADER)" ]; then \
		mkdir -p .deps; \
		nuget install Microsoft.Web.WebView2 -Version $(WEBVIEW2_VERSION) -ExcludeVersion -OutputDirectory .deps; \
	fi
endif

clean:
	rm -rf $(UI_GO_DIR)

embed-resources: $(WEBVIEW2_DEPENDENCY)
	rm -rf $(UI_GO_DIR)
	mkdir -p $(UI_GO_DIR)
ifeq ($(OS),Windows_NT)
	@test -f "$(WEBVIEW2_LOADER)" || { echo "WebView2Loader.dll was not found; set WEBVIEW2_LOADER to the WebView2 SDK loader path" >&2; exit 1; }
	cp "$(WEBVIEW2_LOADER)" $(UI_GO_DIR)/WebView2Loader.dll
endif
	printf '\n' > $(UI_GO_DIR)/placeholder
