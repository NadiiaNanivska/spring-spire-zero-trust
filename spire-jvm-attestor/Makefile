BINARY     := jvm-attestor
CMD_PKG    := ./cmd/jvm-attestor
OUTPUT_DIR := bin

GO         := go
# -buildvcs=false avoids "error obtaining VCS status" when the repo is mounted
# from a Windows drive into WSL/git-bash (git flags it as "dubious ownership").
# Override with `make build GOFLAGS=` if you'd rather fix it via
# `git config --global --add safe.directory <repo-path>`.
GOFLAGS    := -buildvcs=false

# Docker image name (no registry needed for kind load)
IMAGE        ?= spire-jvm-attestor:latest

# Kind cluster name (must match wsldev --name flag)
KIND_CLUSTER ?= kind

# Agent ConfigMap carrying the plugin_checksum the agent verifies at launch
AGENT_CONFIGMAP ?= ../spiffe-spire/base/agent-configmap.yaml

.PHONY: all build test test-race lint vet fmt clean \
        docker-build kind-load dev \
        registry registry-patch registry-down kind-setup \
        docker-push checksum checksum-sync

all: build test

build:
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build $(GOFLAGS) -o $(OUTPUT_DIR)/$(BINARY) $(CMD_PKG)
	chmod 0755 $(OUTPUT_DIR)/$(BINARY)
	sha256sum $(OUTPUT_DIR)/$(BINARY) > $(OUTPUT_DIR)/$(BINARY).sha256
	@echo "Built  : $(OUTPUT_DIR)/$(BINARY)"
	@echo "SHA256 : $$(cat $(OUTPUT_DIR)/$(BINARY).sha256)"

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

## registry: start a local Docker registry on port 5000 and connect it to the Kind network
registry:
	docker inspect kind-registry >/dev/null 2>&1 || \
	  docker run -d --restart=always -p 5000:5000 --name kind-registry registry:2
	docker network connect kind kind-registry 2>/dev/null || true
	@echo "Local registry running at $(LOCAL_REGISTRY)"

## registry-patch: patch containerd on existing Kind nodes to mirror localhost:5000 → kind-registry:5000
##   Run this once after `wsldev up` if the cluster already exists.
registry-patch:
	@echo "Patching containerd mirror on Kind nodes..."
	@for node in $$(kind get nodes --name $(KIND_CLUSTER)); do \
	  echo "  -> $$node"; \
	  docker exec $$node sh -c '\
	    mkdir -p /etc/containerd/certs.d/localhost:5000 && \
	    printf "[host.\"http://kind-registry:5000\"]\n  capabilities = [\"pull\",\"resolve\"]\n  skip_verify = true\n" \
	      > /etc/containerd/certs.d/localhost:5000/hosts.toml'; \
	  docker exec $$node kill -HUP $$(docker exec $$node pgrep -o containerd) 2>/dev/null || true; \
	done
	@echo "Done. Kind nodes now mirror localhost:5000 → kind-registry:5000"

## kind-setup: create a fresh Kind cluster pre-configured with the local registry mirror
kind-setup:
	kind delete cluster --name $(KIND_CLUSTER) 2>/dev/null || true
	kind create cluster --name $(KIND_CLUSTER) --config kind-config.yaml

## registry-down: stop and remove the local registry
registry-down:
	docker rm -f kind-registry 2>/dev/null || true

## docker-build: build image and load it into the local Docker daemon
docker-build:
	docker buildx build \
	  --platform linux/amd64 \
	  --tag $(IMAGE) \
	  --load \
	  .

## kind-load: load the image directly into the Kind cluster (no registry needed)
kind-load:
	kind load docker-image $(IMAGE) --name $(KIND_CLUSTER)

## dev: one-shot — build, load into Kind, and sync the checksum into agent.conf
dev: docker-build kind-load checksum-sync
	@echo "Image $(IMAGE) loaded into Kind cluster '$(KIND_CLUSTER)'"

## docker-push: build multi-arch image (amd64 + arm64) and push directly to the registry
docker-push:
	docker buildx inspect multiarch-builder >/dev/null 2>&1 || \
	  docker buildx create --name multiarch-builder --driver docker-container --use
	docker buildx build \
	  --builder multiarch-builder \
	  --platform linux/amd64,linux/arm64 \
	  --tag $(IMAGE) \
	  --push \
	  .

## checksum: print the SHA-256 of the binary inside the image
checksum:
	@id=$$(docker create $(IMAGE)); \
	 docker cp $$id:/jvm-attestor.sha256 - 2>/dev/null | tar -xO; \
	 docker rm -f $$id >/dev/null

## checksum-sync: write that SHA-256 into the agent ConfigMap's plugin_checksum
# A stale value here does not degrade gracefully: the agent refuses to launch the
# plugin and then exits, so the whole node loses attestation. Never hand-copy it.
checksum-sync:
	@sum=$$($(MAKE) --no-print-directory checksum | tr -d '[:space:]'); \
	 case "$$sum" in *[!0-9a-f]*|"") echo "no checksum in image $(IMAGE); run 'make docker-build' first" >&2; exit 1;; esac; \
	 test -f "$(AGENT_CONFIGMAP)" || { echo "not found: $(AGENT_CONFIGMAP)" >&2; exit 1; }; \
	 sed -i -E 's|(plugin_checksum[[:space:]]*=[[:space:]]*")[0-9a-f]*(")|\1'"$$sum"'\2|' "$(AGENT_CONFIGMAP)"; \
	 grep -q "$$sum" "$(AGENT_CONFIGMAP)" || { echo "plugin_checksum not present in $(AGENT_CONFIGMAP)" >&2; exit 1; }; \
	 echo "plugin_checksum -> $$sum"

clean:
	rm -rf $(OUTPUT_DIR)

help:
	@grep -E '^##' Makefile | sed 's/## //'
