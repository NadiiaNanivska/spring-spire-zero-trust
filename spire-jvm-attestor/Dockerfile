# Stage 1 — build the static Linux binary
FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /jvm-attestor \
      ./cmd/jvm-attestor

# Compute the SHA-256 so the caller can embed it into agent.conf
RUN sha256sum /jvm-attestor | awk '{print $1}' > /jvm-attestor.sha256

# Stage 2 — busybox gives us `cp`, `sh` etc. for the init-container;
# the static binary itself has no libc dependencies.
FROM busybox:musl
COPY --from=builder /jvm-attestor        /jvm-attestor
COPY --from=builder /jvm-attestor.sha256 /jvm-attestor.sha256
ENTRYPOINT ["/jvm-attestor"]
