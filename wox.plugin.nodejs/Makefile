.PHONY: clean build install help publish

help:
	@echo "Available commands:"
	@echo "  make clean      - Remove build artifacts and node_modules"
	@echo "  make build      - Build Node.js plugin"
	@echo "  make install    - Install dependencies"
	@echo "  make publish    - Publish to npm registry"

clean:
	pnpm run clean

install: clean
	pnpm install

build: install
	pnpm run build

publish: build
	pnpm run pub
	rm -rf dist/
