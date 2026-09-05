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
| 3. Jar Integrity | `/proc/<PID>/maps` ∪ `/proc/<PID>/fd`, SHA-256 read through the `/proc` handle | ~200 ms (first), ~15 µs (cached) |

### Jar discovery

The two kernel-attested sources are **unioned**, not tried in turn. A Spring Boot
fat-jar is read with `pread()` rather than mapped, so it appears only in the fd
table — but taking the first non-empty source would let a process hide that table
by mapping one approved jar into its own address space, concealing any extra jar it
holds open. `cmdline` is consulted only when the kernel reports nothing, and a jar
found that way is flagged `maps_verified=false` because the process can rewrite its
own argv ([T1036.011](https://attack.mitre.org/techniques/T1036/011/)).

Jars are read through `/proc/<PID>/fd/<N>` (or `map_files/<range>`), which the
kernel binds to the inode, so a symlink swap or an unlink-and-replace on the
pathname cannot redirect the hash.

## Selectors

```
jvm:debug_clean=true|false
jvm:agent_flags_clean=true|false
jvm:attach_socket_exposed=true|false
jvm:maps_verified=true|false
jvm:hash_via_kernel_handle=true|false
jvm:inode_consistent=true|false
jvm:jar_source=maps|fd|maps+fd|cmdline
jvm:jar_sha256=<hex>              # one per discovered jar
jvm:jar_set_sha256=<hex>          # digest over the whole sorted set
```

The plugin **computes** these values; it never compares them against a reference.
The expected hash is pinned in the SPIRE registration entry, so attestation never
blocks on an external API call. `jar_set_sha256` is what actually pins a workload:
SPIRE matches an entry when its selectors are a *subset* of the workload's, so an
entry pinned only on `jar_sha256` would still match a process that additionally
loaded an attacker's jar.

## Build

```bash
make build
```

This produces two files in `bin/`:

| File | Purpose |
|---|---|
| `bin/jvm-attestor` | Plugin binary (permissions: `0755`) |
| `bin/jvm-attestor.sha256` | SHA256 checksum for `plugin_checksum` |

## Deploy

Copy the binary to the SPIRE agent host and set the `plugin_checksum` field in
`agent.conf` to the value produced by `make build`:

```bash
# Print the checksum value to use in agent.conf
cat bin/jvm-attestor.sha256
# Example output: a3f2b1c4d5e6f7a8...  bin/jvm-attestor

# Deploy the binary
install -m 0755 bin/jvm-attestor /opt/spire/plugins/jvm-attestor
```

```hcl
# agent.conf — use the hex digest from bin/jvm-attestor.sha256
plugin_checksum = "sha256:<hex digest>"
```

The binary must be owned by root and have permissions `0755` (or `0700`) so the
SPIRE agent can execute it but untrusted users cannot replace it.

## Configuration

```hcl
# /etc/spire/agent/agent.conf
plugins {
  WorkloadAttestor "jvm" {
    plugin_cmd      = "/opt/spire/plugins/jvm-attestor"
    plugin_checksum = "sha256:<hex>"

    plugin_data {
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

This is the selector set `wsldev` pins (see `jvmEntrySelectors` in
`wsldev/internal/apps/jvm_register.go`):

```hcl
spiffe://prod.example.org/service/payments {
  selectors = [
    "jvm:debug_clean=true",
    "jvm:agent_flags_clean=true",
    "jvm:maps_verified=true",
    "jvm:hash_via_kernel_handle=true",
    "jvm:jar_sha256=a3f2b1c4d5e6f7a8...",
    "jvm:jar_set_sha256=9e4c7b2d1f0a3e5c...",
  ]
}
```

`jar_set_sha256` is SHA-256 over one `<path>:<sha256>\n` line per discovered jar,
ordered by path. `wsldev` reproduces that format offline in `jarSetDigest`; the two
live in separate Go modules, so mirrored tests pin the wire format.

## Run tests

```bash
go test ./...
```
