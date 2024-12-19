.PHONY: clean build install help

HOSTS_DIR := ../wox.core/resource/hosts

help:
	@echo "Available commands:"
	@echo "  make clean      - Remove build artifacts and cache files"
	@echo "  make build      - Build Python host"
	@echo "  make install    - Install development dependencies"

clean:
	rm -rf python-host.pyz .venv/ __pycache__/ *.pyc python-host/
	find . -type d -name "__pycache__" -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete

install:
	uv cache clean
	uv sync --all-extras

build: install clean
	uv pip freeze > requirements.txt
	uv pip install -r requirements.txt --target python-host
	rm requirements.txt
	cp -r src/wox_plugin_host/*.py python-host/
	uv run python -m zipapp -p "interpreter" python-host
	rm -rf python-host
	mkdir -p $(HOSTS_DIR)
	mv python-host.pyz $(HOSTS_DIR)/python-host.pyz