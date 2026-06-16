.PHONY: clean build install help publish

# The previous SDK build preferred Corepack whenever the shim was present, but
# some local Node/Corepack installs expose `corepack` while `corepack pnpm`
# still crashes. Prefer a working global pnpm first, then fall back to a
# working Corepack shim so package commands keep using an executable CLI.
PNPM ?= $(shell if command -v pnpm >/dev/null 2>&1 && pnpm --version >/dev/null 2>&1; then echo pnpm; elif command -v corepack >/dev/null 2>&1 && corepack pnpm --version >/dev/null 2>&1; then echo "corepack pnpm"; else echo pnpm; fi)

help:
	@echo "Available commands:"
	@echo "  make clean      - Remove build artifacts and node_modules"
	@echo "  make build      - Build Node.js plugin"
	@echo "  make install    - Install dependencies"
	@echo "  make publish    - Publish to npm registry"

clean:
	$(PNPM) run clean

install: clean
	# Make recipes are non-interactive, so tell pnpm it may replace an incompatible node_modules layout without prompting.
	CI=true $(PNPM) install

build: install
	$(PNPM) run build

publish: build
	$(PNPM) run pub
	rm -rf dist/
