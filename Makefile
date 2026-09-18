BINARY  := terraform-provider-nile
VERSION ?= dev
HASH    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
OS_ARCH := $(shell go env GOOS)_$(shell go env GOARCH)
OUT_DIR := bin

LD_FLAGS := -X main.version=$(VERSION)

.PHONY: build install test fmt vet clean

build:
	go build -ldflags "$(LD_FLAGS)" -o $(OUT_DIR)/$(BINARY)_v$(VERSION) .

install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/golden-apple-research/nile/$(VERSION)/$(OS_ARCH)
	cp $(OUT_DIR)/$(BINARY)_v$(VERSION) ~/.terraform.d/plugins/registry.terraform.io/golden-apple-research/nile/$(VERSION)/$(OS_ARCH)/

test:
	go test ./... -v

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf $(OUT_DIR)
