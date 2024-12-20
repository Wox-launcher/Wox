.PHONY: clean build help

# Determine the current platform
ifeq ($(OS),Windows_NT)
    PLATFORM := windows
    ARCH := amd64
    GOARCH := amd64
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Linux)
        PLATFORM := linux
        ARCH := amd64
        GOARCH := amd64
    endif
    ifeq ($(UNAME_S),Darwin)
        PLATFORM := macos
        ARCH := $(shell uname -m)
        ifeq ($(ARCH),x86_64)
            GOARCH := amd64
        else
            GOARCH := $(ARCH)
        endif
    endif
endif

RELEASE_DIR := ../release

help:
	@echo "Available commands:"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make build      - Build Flutter UI for current platform ($(PLATFORM))"

clean:
	rm -rf $(RELEASE_DIR)

build: clean
ifeq ($(PLATFORM),windows)
	CGO_ENABLED=1 GOOS=windows GOARCH=$(GOARCH) go build -ldflags "-H windowsgui -s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-windows-$(GOARCH).exe
endif
ifeq ($(PLATFORM),linux)
	CGO_ENABLED=1 GOOS=linux GOARCH=$(GOARCH) go build -ldflags "-s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-linux-$(GOARCH)
endif
ifeq ($(PLATFORM),macos)
	CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) CGO_CFLAGS="-mmacosx-version-min=10.15" CGO_LDFLAGS="-mmacosx-version-min=10.15" go build -ldflags "-s -w -X 'wox/util.ProdEnv=true'" -o $(RELEASE_DIR)/wox-mac-$(GOARCH)
endif 