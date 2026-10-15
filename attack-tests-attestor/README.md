# JVM Attestor Attack / Resilience Tests

Live functional tests for SPIRE `jvm` WorkloadAttestor plugin (`spire-jvm-attestor`) on a kind cluster with overlay **custom-jvm**.

## Purpose (section 1.1)

Prove on a live deployment that the plugin:

1. **Blocks SVID issuance** for attacks it is designed to detect (all branches of 3 defense levels).
2. **Holds** against user-space `/proc` spoofing and symlink tricks: jars are read through a `/proc` handle bound to the inode, not by resolving their pathname.
3. **Detects extra code**: a jar the deployment never approved changes `jvm:jar_set_sha256`, so the workload no longer matches its registration entry — a per-jar hash alone would not catch this, since SPIRE matches on a selector *subset*. Discovery **unions** `maps` and the fd table rather than stopping at the first non-empty source, so a process cannot hide its descriptor table by mapping an approved jar into its own address space (`bypass-mmap-shadow.sh`).
4. **Survives** stress / error inputs without crashing the SPIRE agent.

## Prerequisites

- JVM plugin image built + loaded (`make dev` in `spire-jvm-attestor/`) and its
  `plugin_checksum` in `spiffe-spire/base/agent-configmap.yaml` refreshed via
  `make checksum` (the agent refuses a plugin whose checksum doesn't match)
- kind cluster with SPIRE (`wsldev spire deploy --attestor custom-jvm`)
- `payments-service` and `orders-service` deployed (`wsldev app deploy payments orders`),
  which also registers the JVM workloads (jar_sha256 entries) via `wsldev`
- `kubectl`, `curl`, `jq`, `bash`
- `wsldev` built or on `PATH`

## Quick start

```bash
cd attack-tests-attestor
chmod +x *.sh
./run-all.sh
```

Partial run:

```bash
./run-all.sh --tests attack-jar-unknown.sh,attack-cp-classpath.sh
./run-all.sh --skip-setup --tests attack-tamper-flags.sh
```

## Test matrix

| Script | Level | Attack / scenario | Expected outcome |
|--------|-------|-------------------|------------------|
| `attack-tamper-flags.sh` | 2 | Boot-safe dangerous cmdline flags (jdwp, `-Xdebug`, attach, jmxremote) | SPIRE identity issuance denied; pod remains Running when the JVM supports the flag |
| `attack-tamper-env.sh` | 2 | All 4 dangerous env vars | SPIRE identity issuance denied; pod remains Running |
| `attack-attach-socket.sh` | 2 | `.java_pid` Attach socket | `FailedPrecondition`, SVID denied |
| `attack-jar-unknown.sh` | 3 | Tampered / unapproved jar hash (not in allow-list) | computed artifact selectors do not match registration ⇒ SVID denied |
| `attack-cp-classpath.sh` | B | Classpath launch (no `-jar`) with an attacker jar ahead of the app jar | both jars discovered via `fd`; approved `jar_sha256` still present but `jar_set_sha256` no longer matches ⇒ **SVID denied** |
| `bypass-symlink.sh` | B | Jar pathname redirected to an attacker decoy (`ln -s decoy.jar payments-service.jar`) | the approved workload remains usable; per-process selector values are not logged |
| `bypass-mmap-shadow.sh` | B | Extra classpath jar **plus** a `FileChannel.map` of the approved jar, to make `maps` answer first and hide the fd table | extra classpath jar scenario ⇒ **SVID denied** (runtime selectors are not logged) |
| `dos-large-jar.sh` | D | Large jar SHA-256 stress | Agent survives, latency logged |

## Methodology

### Deny-first: attack the pod *before* it holds a valid SVID

SPIRE never revokes an already-issued SVID — it only refuses to *renew* one whose
selectors stopped matching. On top of that, `orders` pools its mTLS connection to
`payments`, and a peer certificate is only checked at TLS handshake. So a workload
compromised *in place* keeps serving valid mTLS on its cached SVID + pooled
connection until the SVID TTL expires and a fresh handshake happens (~a minute).
Asserting the denial (`orders -> payments` non-2xx) too early therefore produced
**false FAILs**.

The tests are now **deny-first**: they arrange for the *compromised* payments pod to
be the one that fetches its first SVID, so denial is deterministic and immediate
rather than TTL-bounded. How the artifact is planted depends on where it lives:

| Artifact location | Deny-first mechanism |
|---|---|
| Pod-local file (attach socket) | Deploy a payments **variant** whose entrypoint plants the artifact *before* `exec java`, with `strategy: Recreate` so the clean pod is torn down first (`write_payments_variant_manifest`). |
| Deployment spec (dangerous flag/env) | Apply the tampered deployment with `strategy: Recreate`; a soft wait (`wait_deployment_settled`) tolerates a JVM that crashes on a bad `-javaagent`. |
| SPIRE server (bogus jar-hash entry) | Re-pin the entry, then `delete` the payments pod (`delete_payments_pod_and_wait`); the replacement is denied on its first fetch. |

### Verification signals

1. **Agent logs** — `collect-attack-logs.sh` greps for JVM selectors and errors.
2. **Functional mTLS** — probe `orders -> payments` via in-cluster curl (`orders_create_from_pod`).
3. **SPIRE entries** — entries are created by `wsldev` (`wsldev spire register-jvm`, also run automatically by `wsldev app deploy`) and require `jvm:has_javaagent=false`, `jvm:has_jdwp=false`, `jvm:java_tool_options_set=false`, `jvm:attach_socket=false`, `jvm:subject_match=true`, `jvm:signed_by=<fingerprint>`, `jvm:jar_sha256=<hash>` and `jvm:jar_set_sha256=<digest>` (see `jvmEntrySelectors` in `wsldev/internal/apps/jvm_register.go`). The plugin only *computes* these values and never hard-fails on a dangerous flag/env/attach-socket; pinning the launch-facts selectors `=false` in the entry is what actually makes SPIRE deny a tampered process. The set digest is the one that actually pins the workload against extra jars — see `jarSetDigest` in the same file.

### Result statuses

| Status | Meaning |
|--------|---------|
| `PASS` | Defense behaved as designed |
| `FAIL` | Attack succeeded when it should not |
| `LIMITATION (expected)` | Documented bypass boundary (not a plugin bug) |

## Output

```
results/run-<timestamp>/
  baseline/              # clean-state agent logs
  baseline.env
  spire-entries.txt
  <test-name>/           # per-test logs + meta.env
  summary.md             # PASS/FAIL/LIMITATION table
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `K8S_NAMESPACE` | `spire` | Target namespace |
| `SETTLE_SEC` | `30` | Wait after deploy/restart |
| `LOG_SINCE` | `300s` | Agent log window |
| `LARGE_JAR_MB` | `200` | DoS test append size |
| `DOS_LATENCY_THRESHOLD_US` | `60000000` | Max acceptable jar_hash_us |
| `SUBTEST_LOG_SINCE` | `120s` | Per-subtest agent log window |
| `MTLS_OK_RETRIES_AFTER_AGENT_RESTART` | `24` | mTLS ok retries after agent restart |
| `WSLDEV_BIN` | auto | Path to wsldev |

## Checker ↔ attack map

```
Level 2 anti-tamper (launch-facts) -> /proc/cmdline flags, /proc/environ, Attach socket
Level 3 jar-hash                   -> jar discovery unions /proc/<pid>/maps and /proc/<pid>/fd,
                                       falling back to (unverified) cmdline only when the kernel
                                       reports nothing; SHA-256 selectors are evaluated by SPIRE
                                       (expected value enforced by the SPIRE registration entry)
```

> **No anti-debug checker.** The plugin does not read `/proc/<pid>/status` or
> `TracerPid` at all — there is no `debug_clean`/`tracer_pid` selector. A ptrace'd
> JVM attests exactly like a clean one. `attack-antidebug.sh` was removed from this
> suite for that reason; reintroduce it only once (if ever) a checker for this exists.

> **Enforcement model (updated):** the `jvm` plugin only *computes* facts — it
> never hard-fails on a dangerous `-javaagent`/JDWP/env var, and `block_on_attach_socket`
> just makes the launch-facts checker return no selectors instead of an error. Denial
> for Level 2 attacks works only because the SPIRE **registration entry** (populated by
> `wsldev spire register-jvm`) pins `jvm:has_javaagent=false`, `jvm:has_jdwp=false`,
> `jvm:java_tool_options_set=false` and `jvm:attach_socket=false` — once the real value
> flips to `true` (or the selector goes missing under `block_on_attach_socket`), SPIRE's
> subset match fails and the workload is denied. The jar hash works the same way: the
> plugin computes `jvm:jar_sha256=<hash>`; the entry pins the approved value, so a
> tampered or unapproved jar simply fails to match. This is why tests assert externally
> observable identity/mTLS outcomes rather than per-process selector log lines. The
> plugin logs the enabled modules at configuration time only.

See [spire-jvm-attestor/README.md](../spire-jvm-attestor/README.md) for plugin design.
