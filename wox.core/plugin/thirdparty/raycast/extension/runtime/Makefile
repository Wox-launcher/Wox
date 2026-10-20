.PHONY: build test
PNPM ?= pnpm
build:
	$(PNPM) install --frozen-lockfile --ignore-scripts
	$(PNPM) run build
test: build
	$(PNPM) test
