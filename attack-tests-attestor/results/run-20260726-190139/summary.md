# JVM Attestor Attack Test Summary

Run: `20260726-190139`

| Test | Expected | Status | Evidence |
|------|----------|--------|----------|
| level1-antidebug | PASS | PASS | mtls-deny:http-500 |
| level2-tamper-flags | PASS | PASS | attestor-log:payments-service-65ddcbb8-2k2wf;mtls-deny:http-500;attestor-log:payments-service-65ddcbb8-2k2wf;mtls-deny:http-500;attestor-log:payments-service-768d498d-gnkcw;mtls-deny:http-500;attestor-log:payments-service-85fb8d7c8b-l255b;mtls-deny:http-500;attestor-log:payments-service-bf7b45c48-ttc5w;mtls-deny:http-500;attestor-log:payments-service-8668f5887b-tdsj5;mtls-deny:http-500 |
| level2-tamper-env | PASS | PASS | attestor-log:payments-service-6b454889fd-vsn5w;mtls-deny:http-500;attestor-log:payments-service-6c4c677dfb-62h7s;mtls-deny:http-500;attestor-log:payments-service-5cd6f846b7-bcklk;mtls-deny:http-500;attestor-log:payments-service-c9c49c76d-5mbvh;mtls-deny:http-500 |
| level2-attach-socket | PASS | FAIL | assertion failed; see logs in /mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/attack-tests-attestor/results/run-20260726-190139/level2-attach-socket |
| level3-jar-unknown | PASS | PASS | jar-hash-changed:a13a43ae91e70b29;attestor-log:payments-service-668b57467c-dmqsd;mtls-deny:http-500 |
| bypass-inmemory | LIMITATION | LIMITATION (expected) | all assertions passed |
| bypass-proc-spoof | PASS | FAIL | assertion failed; see logs in /mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/attack-tests-attestor/results/run-20260726-190139/bypass-proc-spoof |
| bypass-symlink | PASS | FAIL | assertion failed; see logs in /mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/attack-tests-attestor/results/run-20260726-190139/bypass-symlink |
| dos-large-jar | PASS | PASS | agent-running;jar-hash-latency-high:0us;dos-survived |

## Totals

- Tests run: 9
- Failures: 3

Status legend: **PASS** = defense worked; **FAIL** = defense missed; **LIMITATION (expected)** = documented bypass boundary.
