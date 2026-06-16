.PHONY: clean build install help sdk

HOSTS_DIR := ../wox.core/resource/hosts
SDK_DIR := ../wox.plugin.nodejs

# GNU Make can run this subdirectory build directly from Windows shells where
# Git usr/bin is missing, so expose the POSIX helpers used by mkdir/cp/rm here
# instead of relying on the root Makefile to have normalized PATH first.
ifeq ($(OS),Windows_NT)
    GIT_USR_BIN := $(patsubst %/bin/sh.exe,%/usr/bin,$(SHELL))
    ifneq ($(GIT_USR_BIN),$(SHELL))
        export PATH := $(GIT_USR_BIN);$(PATH)
    endif
endif

# The previous host build preferred Corepack whenever the shim was present, but
# some local Node/Corepack installs expose `corepack` while `corepack pnpm`
# still crashes. Prefer a working global pnpm first, then fall back to a
# working Corepack shim so nested root builds keep using an executable CLI.
PNPM ?= $(shell if command -v pnpm >/dev/null 2>&1 && pnpm --version >/dev/null 2>&1; then echo pnpm; elif command -v corepack >/dev/null 2>&1 && corepack pnpm --version >/dev/null 2>&1; then echo "corepack pnpm"; else echo pnpm; fi)

help:
	@echo "Available commands:"
	@echo "  make clean      - Remove build artifacts and node_modules"
	@echo "  make build      - Build Node.js host"
	@echo "  make install    - Install dependencies"

clean:
	$(PNPM) run clean

sdk:
	# The host bundles SDK runtime code, so build the local SDK before pnpm resolves the file dependency and captures its dist output.
	$(MAKE) -C $(SDK_DIR) build

install: sdk clean
	# Make recipes are non-interactive, so tell pnpm it may replace an incompatible node_modules layout without prompting.
	CI=true $(PNPM) install

build: install
	$(PNPM) run build
	mkdir -p $(HOSTS_DIR)
	cp dist/index.js $(HOSTS_DIR)/node-host.js
	rm -rf dist/
