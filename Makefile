.PHONY: build install clean help

BINARY_NAME=protoc-go-inject
GO=go
VERSION=v1.0.0

help:
	@echo "Available targets:"
	@echo "  build   - Build the protoc-go-inject binary"
	@echo "  install - Install the binary to GOPATH/bin"
	@echo "  clean   - Remove built binary"
	@echo "  help    - Show this help message"

build:
	$(GO) build -ldflags "-X main.Version=$(VERSION)" -o $(BINARY_NAME)

install: build
	$(GO) install

clean:
	rm -f $(BINARY_NAME)
	rm -f $(GOPATH)/bin/$(BINARY_NAME)

# Default target
.DEFAULT_GOAL := help
