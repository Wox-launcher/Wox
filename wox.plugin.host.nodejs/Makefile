.PHONY: clean build install help

HOSTS_DIR := ../wox.core/resource/hosts

help:
	@echo "Available commands:"
	@echo "  make clean      - Remove build artifacts and node_modules"
	@echo "  make build      - Build Node.js host"
	@echo "  make install    - Install dependencies"

clean:
	pnpm run clean

install: clean
	pnpm install

build: install
	pnpm run build
	mkdir -p $(HOSTS_DIR)
	cp dist/index.js $(HOSTS_DIR)/node-host.js
	rm -rf dist/