BINARY  := terraform-provider-nile
VERSION ?= dev
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)
OUT_DIR := bin

LD_FLAGS := -X main.version=$(VERSION)

.PHONY: build install test fmt vet smoke clean

build:
	mkdir -p $(OUT_DIR)
	go build -ldflags "$(LD_FLAGS)" -o $(OUT_DIR)/$(BINARY)_v$(VERSION) .

install: build
	@echo "$(VERSION)" | grep -Eq '^v?[0-9]+\.[0-9]+\.[0-9]+' || { echo "VERSION must be a semver version, e.g. 'make install VERSION=0.1.0'"; exit 1; }
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/golden-apple-research/nile/$(VERSION)/$(OS_ARCH)
	cp $(OUT_DIR)/$(BINARY)_v$(VERSION) ~/.terraform.d/plugins/registry.terraform.io/golden-apple-research/nile/$(VERSION)/$(OS_ARCH)/

test:
	go test ./... -v

fmt:
	gofmt -w .

vet:
	go vet ./...

smoke:
	tests/smoke/run.sh

clean:
	rm -rf $(OUT_DIR)
