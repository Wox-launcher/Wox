.PHONY: clean build install help

HOSTS_DIR := ../wox.core/resource/hosts

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
	uv run shiv --compressed -c run -o $(HOSTS_DIR)/python-host.pyz .