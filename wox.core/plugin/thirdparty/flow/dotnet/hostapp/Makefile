.PHONY: build

# Five levels up from hostapp is wox.core.
HOSTS_DIR := ../../../../../resource/hosts/flow
PUBLISH_DIR := out

# GNU Make can run this subdirectory build directly from Windows shells where
# Git usr/bin is missing, so expose the POSIX helpers used by mkdir/cp here
# instead of relying on the root Makefile to have normalized PATH first.
ifeq ($(OS),Windows_NT)
    GIT_USR_BIN := $(patsubst %/bin/sh.exe,%/usr/bin,$(SHELL))
    ifneq ($(GIT_USR_BIN),$(SHELL))
        export PATH := $(GIT_USR_BIN);$(PATH)
    endif
endif

# Program Files dotnet is often the runtime only. Prefer an SDK that can publish,
# then the SDK unpacked for local Wox builds.
DOTNET := $(shell if command -v dotnet >/dev/null 2>&1 && dotnet --version >/dev/null 2>&1; then command -v dotnet; elif [ -n "$$LOCALAPPDATA" ] && [ -x "$$LOCALAPPDATA/wox-dotnet-sdk/dotnet.exe" ]; then printf '%s\n' "$$LOCALAPPDATA/wox-dotnet-sdk/dotnet.exe"; fi)

build:
	@if [ -z "$(DOTNET)" ]; then echo ".NET SDK is required to publish the flow host. Install it or unpack it to %LOCALAPPDATA%\\wox-dotnet-sdk." >&2; exit 1; fi
	mkdir -p "$(PUBLISH_DIR)" "$(HOSTS_DIR)"
	# Publish beside the project first, then copy the loader into resource/hosts/flow.
	"$(DOTNET)" publish -c Release -o "$(PUBLISH_DIR)" --nologo
	cp "$(PUBLISH_DIR)/Wox.Flow.DotNetHost.dll" "$(HOSTS_DIR)/"
	cp "$(PUBLISH_DIR)/Wox.Flow.DotNetHost.deps.json" "$(HOSTS_DIR)/"
	cp "$(PUBLISH_DIR)/Wox.Flow.DotNetHost.runtimeconfig.json" "$(HOSTS_DIR)/"
	cp "$(PUBLISH_DIR)/Flow.Launcher.Plugin.dll" "$(HOSTS_DIR)/"
	cp "$(PUBLISH_DIR)/JetBrains.Annotations.dll" "$(HOSTS_DIR)/"
