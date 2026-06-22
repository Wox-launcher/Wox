.PHONY: clean build install help

HOSTS_DIR := ../wox.core/resource/hosts
SDK_DIR := ../wox.plugin.python

# GNU Make can run this subdirectory build directly from Windows shells where
# Git usr/bin is missing, so expose the POSIX helpers used by rm here instead of
# relying on the root Makefile to have normalized PATH first.
ifeq ($(OS),Windows_NT)
    GIT_USR_BIN := $(patsubst %/bin/sh.exe,%/usr/bin,$(SHELL))
    ifneq ($(GIT_USR_BIN),$(SHELL))
        export PATH := $(GIT_USR_BIN);$(PATH)
    endif
endif

help:
	@echo "Available commands:"
	@echo "  make clean      - Remove build artifacts and cache files"
	@echo "  make build      - Build Python host"
	@echo "  make install    - Install development dependencies"
	@echo "  make lint       - Run linting"
	@echo "  make format     - Run formatting"

clean:
	rm -rf python-host.pyz python-host/

install:
	uv cache clean
	uv sync --all-extras

lint:
	uv run ruff check src
	uv run mypy src

format:
	uv run ruff format src

build: clean install lint format
	# uv sources keep the development environment on the local SDK, but shiv
	# resolves its own wheel set; pass the SDK path explicitly so the bundled
	# host never falls back to the published PyPI SDK.
	uv run shiv --compressed -c run -o $(HOSTS_DIR)/python-host.pyz . $(SDK_DIR)
