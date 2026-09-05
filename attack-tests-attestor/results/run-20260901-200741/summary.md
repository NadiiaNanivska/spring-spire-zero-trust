# JVM Attestor Attack Test Summary

Run: `20260901-200741`

| Test | Expected | Status | Evidence |
|------|----------|--------|----------|
| level1-antidebug | PASS | PASS | mtls-deny:http-500 |
| level2-tamper-flags | PASS | PASS | attestor-log:payments-service-7c964d654d-hlrjq:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-59d48f9c64-sjz5w:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-587d467bd6-8jtfz:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-765b698db4-lkhzq:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-c6bfc7798-wlfsz:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-55bf94b84-9t55d:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-75c9cd87d4-7t56v:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;flags-detected:7/7 |
| level2-tamper-env | PASS | PASS | attestor-log:payments-service-796dcd8474-6qrd2:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-75759f44d-gn7wb:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-544546cb75-4blp9:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;attestor-log:payments-service-76d7cf8c88-w59hk:PID attested to have selectors (agent_flags_clean=false);mtls-deny:http-500;env-vars-detected:4/4 |
| level2-attach-socket | PASS | PASS | attestor-log:payments-service-9cfbc9f78-r694d:JVM Attach API socket exposed at /proc/15967/root/tmp/.java_pid1 refusing attestation;svid-denied:payments-service-9cfbc9f78-r694d |
| level3-jar-unknown | PASS | PASS | jar-hash-changed:9575a492b5d01b48;attestor-log:payments-service-f9cbd4499-q2rmc:PID attested to have selectors (jar_sha256=);mtls-deny:http-500 |
| bypass-cp-classpath | PASS | PASS | jvm-plugin-active;discovery:fd;approved-jar-selector-still-present;jar-set-digest-changed;no-identity-issued:pid-20008;orders->payments:http-500 |
| bypass-symlink | PASS | PASS | decoy-symlink-in-place;hash-follows-fd-not-symlink:cbd54b0bc443f780;jar-set-digest-unchanged;discovery:fd+kernel-handle;mtls-ok:http-200 |
| bypass-mmap-shadow | PASS | FAIL | assertion failed; see logs in /mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/attack-tests-attestor/results/run-20260901-200741/bypass-mmap-shadow |
| dos-large-jar | PASS | PASS | agent-running;jar-hash-latency-ok:185104us;jar-hash-changed:d434f94dea4714dd;dos-survived |

## Totals

- Tests run: 9
- Failures: 1

Status legend: **PASS** = defense worked; **FAIL** = defense missed; **LIMITATION (expected)** = documented bypass boundary.
