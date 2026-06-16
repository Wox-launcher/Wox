.PHONY: help smoke

SMOKE_FILTER ?= $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

help:
	@echo "Available commands:"
	@echo "  make smoke                              Run the desktop smoke E2E flow"
	@echo "  make smoke \"Keyboard navigation\"        Run smoke tests filtered by name"

smoke:
	dart run bin/run.dart smoke "$(SMOKE_FILTER)"

%:
	@:
