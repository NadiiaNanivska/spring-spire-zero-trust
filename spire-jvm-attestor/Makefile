BINARY     := jvm-attestor
CMD_PKG    := ./cmd/jvm-attestor
OUTPUT_DIR := bin

GO         := go
GOFLAGS    :=

.PHONY: all build test test-race lint vet fmt clean

all: build test

build:
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build $(GOFLAGS) -o $(OUTPUT_DIR)/$(BINARY) $(CMD_PKG)
	@echo "Built $(OUTPUT_DIR)/$(BINARY)"

test:
	$(GO) test $(GOFLAGS) ./...

test-race:
	$(GO) test -race $(GOFLAGS) ./...

test-verbose:
	$(GO) test -v $(GOFLAGS) ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

lint: vet
	@test -z "$$(gofmt -l .)" || (echo "Run 'make fmt' to fix formatting:"; gofmt -l .; exit 1)

## manifest: generate a test hash manifest from a local jar
## Usage: make manifest JAR=/path/to/your.jar OUT=jvm-hashes.json
manifest:
	@test -n "$(JAR)" || (echo "Usage: make manifest JAR=/path/to/file.jar"; exit 1)
	bash scripts/generate-manifest.sh $(JAR) $(or $(OUT),jvm-hashes.json)

clean:
	rm -rf $(OUTPUT_DIR)

help:
	@grep -E '^##' Makefile | sed 's/## //'
