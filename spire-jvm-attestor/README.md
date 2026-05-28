# SPIRE JVM Process Attestor

A custom [SPIRE](https://github.com/spiffe/spire) `WorkloadAttestor` plugin that verifies
JVM process integrity at the Linux kernel level before issuing an SVID.

Addresses [spiffe/spire#2426](https://github.com/spiffe/spire/issues/2426): the standard
`unix` attestor identifies all JVM processes by the same binary path (`/usr/bin/java`),
making it impossible to distinguish between workloads by code identity.

## How it works

The plugin reads `/proc/<PID>/*` — kernel-authoritative data that cannot be spoofed
from user-space — and returns SPIRE selectors that reflect the actual runtime state
of the process:

| Check | Source | Cost |
|---|---|---|
| 1. Anti-Debug | `/proc/<PID>/status → TracerPid` | ~10 µs |
| 2. Anti-Tamper | `/proc/<PID>/cmdline`, `environ`, Attach socket | ~110 µs |
| 3. Jar Integrity | `/proc/<PID>/maps` + SHA-256 vs CI manifest | ~200 ms (first), ~15 µs (cached) |

## Selectors

```
jvm:debug_clean=true|false
jvm:agent_flags_clean=true|false
jvm:attach_socket_exposed=true|false
jvm:maps_verified=true
jvm:inode_consistent=true|false
jvm:jar_sha256:<hex>
```

## Build

```bash
go build -o jvm-attestor ./cmd/jvm-attestor
```

## Configuration

```hcl
# /etc/spire/agent/agent.conf
plugins {
  WorkloadAttestor "jvm" {
    plugin_cmd      = "/opt/spire/plugins/jvm-attestor"
    plugin_checksum = "sha256:<hex>"

    plugin_data {
      hash_manifest_path     = "/etc/spire/jvm-hashes.json"
      block_on_attach_socket = true
    }
  }
}
```

## CI/CD manifest generation

```bash
./scripts/generate-manifest.sh /app/payments-service.jar jvm-hashes.json
```

## SPIRE registration entry example

```hcl
spiffe://prod.example.org/service/payments {
  selectors = [
    "jvm:debug_clean=true",
    "jvm:agent_flags_clean=true",
    "jvm:attach_socket_exposed=false",
    "jvm:maps_verified=true",
    "jvm:inode_consistent=true",
    "jvm:jar_sha256:a3f2b1c4d5e6f7a8...",
  ]
}
```

## Run tests

```bash
go test ./...
```
