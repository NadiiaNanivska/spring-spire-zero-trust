BINARY     := jvm-attestor
CMD_PKG    := ./cmd/jvm-attestor
OUTPUT_DIR := bin

GO         := go
# Disable VCS stamping for WSL mounts that Git flags as dubious ownership.
GOFLAGS    := -buildvcs=false

IMAGE        ?= spire-jvm-attestor:latest

KIND_CLUSTER ?= kind

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

manifest:
	@test -n "$(JAR)" || (echo "Usage: make manifest JAR=/path/to/file.jar"; exit 1)
	bash scripts/generate-manifest.sh $(JAR) $(or $(OUT),jvm-hashes.json)

registry:
	docker inspect kind-registry >/dev/null 2>&1 || \
	  docker run -d --restart=always -p 5000:5000 --name kind-registry registry:2
	docker network connect kind kind-registry 2>/dev/null || true
	@echo "Local registry running at $(LOCAL_REGISTRY)"

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

kind-setup:
	kind delete cluster --name $(KIND_CLUSTER) 2>/dev/null || true
	kind create cluster --name $(KIND_CLUSTER) --config kind-config.yaml

registry-down:
	docker rm -f kind-registry 2>/dev/null || true

docker-build:
	docker buildx build \
	  --platform linux/amd64 \
	  --tag $(IMAGE) \
	  --load \
	  .

kind-load:
	kind load docker-image $(IMAGE) --name $(KIND_CLUSTER)

dev: docker-build kind-load checksum-sync
	@echo "Image $(IMAGE) loaded into Kind cluster '$(KIND_CLUSTER)'"

docker-push:
	docker buildx inspect multiarch-builder >/dev/null 2>&1 || \
	  docker buildx create --name multiarch-builder --driver docker-container --use
	docker buildx build \
	  --builder multiarch-builder \
	  --platform linux/amd64,linux/arm64 \
	  --tag $(IMAGE) \
	  --push \
	  .

checksum:
	@id=$$(docker create $(IMAGE)); \
	 docker cp $$id:/jvm-attestor.sha256 - 2>/dev/null | tar -xO; \
	 docker rm -f $$id >/dev/null

# A stale checksum prevents the agent from starting the plugin.
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
