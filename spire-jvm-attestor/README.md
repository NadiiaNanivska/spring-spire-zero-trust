# SPIRE JVM Process Attestor

A custom SPIRE `WorkloadAttestor` plugin that adds Java-specific runtime selectors to workload attestation on Linux. It is intended to let SPIRE registration entries distinguish JVM workloads by selected launch facts and by the JAR artifacts observed for the process.

The plugin observes process state through `/proc/<PID>`. This relies on the integrity of the host kernel and SPIRE agent; it is not a defense against a compromised node or kernel.

## Checks

Each checker can be enabled independently through `check_jdk`, `check_launch`, `check_jar_hash`, and `check_provenance`. All are enabled by default when omitted. Provenance verification requires JAR hashing.

| Checker | Observed information | Important limitation |
|---|---|---|
| JDK | Major Java version read from the executable's JDK `release` file | Reports a version; does not cryptographically verify JDK binaries |
| Launch facts | Java agent/JDWP flags in `cmdline`, JVM option environment variables, Attach socket | Does not detect every possible runtime instrumentation or JMX configuration |
| JAR hashing | SHA-256 of discovered JARs, a digest of the sorted JAR set, discovery source, kernel-handle and inode consistency facts | Uses a path-based fallback when a kernel handle is unavailable; the resulting selectors expose this weaker state |
| Provenance | Ed25519 signature over the JAR SHA-256 digest, verified against configured public keys | A valid signature proves that the trusted key signed the digest; it does not independently prove source-code or CI build provenance |

## JAR discovery and integrity selectors

JAR discovery unions entries from `/proc/<PID>/maps` and `/proc/<PID>/fd`. This prevents the implementation from ignoring open descriptors merely because at least one mapped JAR was found. If neither kernel-observed source finds a JAR, the checker can fall back to `cmdline`; that result is explicitly marked `maps_verified=false`.

For each discovered JAR the checker emits `jar_sha256`. It also emits `jar_set_sha256`, computed from sorted `<path>:<sha256>\n` records. The additional facts `maps_verified`, `hash_via_kernel_handle`, and `inode_consistent` describe how the files were discovered and read. These are observations, not independent proof that the whole JVM runtime is uncompromised.

## Selectors

```
jvm:java_version=<major-version>
jvm:has_javaagent=true|false
jvm:has_jdwp=true|false
jvm:java_tool_options_set=true|false
jvm:attach_socket=true|false
jvm:maps_verified=true|false
jvm:hash_via_kernel_handle=true|false
jvm:inode_consistent=true|false
jvm:jar_source=maps|fd|maps+fd|cmdline
jvm:jar_sha256=<hex>              # one per discovered JAR
jvm:jar_set_sha256=<hex>          # digest over the whole sorted set
jvm:signed_by=<public-key-fingerprint>
jvm:subject_match=true
```

The plugin computes selectors; it does not compare them with an allow-list. SPIRE registration entries perform that comparison. Because SPIRE selector matching is subset-based, the entry must pin every security-relevant fact that is required for the workload. In particular, an entry that does not require `maps_verified=true`, `hash_via_kernel_handle=true`, and `inode_consistent=true` may accept a workload whose JAR discovery or hashing used a weaker path.

The `wsldev` registration currently pins:
- `jvm:java_version=17` (the Java major version);
- `jvm:has_javaagent=false`, `jvm:has_jdwp=false`, `jvm:java_tool_options_set=false`, and `jvm:attach_socket=false`;
- `jvm:maps_verified=true`, `jvm:hash_via_kernel_handle=true`, and `jvm:inode_consistent=true`;
- the expected `jvm:jar_sha256`, `jvm:jar_set_sha256`, `jvm:signed_by`, and `jvm:subject_match=true`.

These selectors intentionally make an absent fact or a weaker observation fail to match the registered workload.

## Attach socket behavior

When `block_on_attach_socket=true`, detection of an Attach socket makes the launch-facts checker return no selectors. SPIRE then cannot match the registration entry that requires the clean launch selectors. The decision is enforced by registration matching, not by a separate cryptographic property of the socket.

## Configuration

```hcl
plugins {
  WorkloadAttestor "jvm" {
    plugin_cmd      = "/opt/spire/plugins/jvm-attestor"
    plugin_checksum = "sha256:<hex>"

    plugin_data {
      block_on_attach_socket = true
      check_jdk        = true
      check_launch     = true
      check_jar_hash   = true
      check_provenance = true
    }
  }
}
```

## Build and test

```bash
make build
go test ./...
```

Build the plugin binary and configure its checksum in the SPIRE Agent configuration before deployment. The public Ed25519 key used for JAR signature verification must be supplied to the agent configuration; the corresponding private key must remain outside the repository and be protected as a signing secret.
