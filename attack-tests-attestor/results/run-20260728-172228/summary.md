# JVM Attestor Attack Test Summary

Run: `20260728-172228`

| Test | Expected | Status | Evidence |
|------|----------|--------|----------|
| level1-antidebug | PASS | PASS | mtls-deny:http-500 |
| level2-tamper-flags | PASS | PASS | attestor-log:payments-service-579768758b-pf47f:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-579768758b-pf47f:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-5bdb9d7fd6-zr4h6:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-5d676fbb64-4hjkp:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-8494df4b6-rhhcj:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-7cdff8c794-96nhg:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500 |
| level2-tamper-env | PASS | PASS | attestor-log:payments-service-7d47bdcf86-6tx8m:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-779b8489bd-8drlp:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-576d6c5c79-2pqqx:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-57d56f6988-mb252:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500 |
| level2-attach-socket | PASS | PASS | attestor-log:payments-service-644c598dc5-cfxzf:JVM Attach API socket exposed at /proc/36570/root/tmp/.java_pid1 refusing attestation |
| level3-jar-unknown | PASS | PASS | jar-hash-changed:a13a43ae91e70b29;attestor-log:payments-service-65956c76c9-xq22p:PID attested to have selectors (jar_sha256=);mtls-deny:http-500 |
| bypass-inmemory | LIMITATION | LIMITATION (expected) | extra-cp-jar-unverified;svid-issued-despite-extra-cp;orders->payments:http-200 |
| bypass-symlink | PASS | PASS | mtls-ok:http-200 |
| dos-large-jar | PASS | PASS | agent-running;jar-hash-latency-ok:183864us;jar-hash-changed:468c2bf138bfefcf;dos-survived |

## Totals

- Tests run: 8
- Failures: 0

Status legend: **PASS** = defense worked; **FAIL** = defense missed; **LIMITATION (expected)** = documented bypass boundary.
