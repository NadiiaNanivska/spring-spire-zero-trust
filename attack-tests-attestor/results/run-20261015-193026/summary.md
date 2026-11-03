# JVM Attestor Attack Test Summary

Run: `20261015-193026`

| Test | Expected | Status | Evidence |
|------|----------|--------|----------|
| level2-tamper-flags | PASS | PASS | attestor-log:payments-service-58c7bbf6d8-pqzzs:No identity issued (No identity issued);mtls-deny:http-500;attestor-log:payments-service-65ddcbb8-rqzsc:No identity issued (No identity issued);mtls-deny:http-500;attestor-log:payments-service-7d8967d7cd-tr4jm:No identity issued (No identity issued);mtls-deny:http-500;attestor-log:payments-service-768d498d-t8xwv:No identity issued (No identity issued);mtls-deny:http-500;attestor-log:payments-service-85fb8d7c8b-b2q4v:No identity issued (No identity issued);mtls-deny:http-500;flags-detected:5/5 |
| level2-tamper-env | PASS | PASS | attestor-log:payments-service-6bff94bb6b-s58ks:No identity issued (No identity issued);mtls-deny:http-500;attestor-log:payments-service-764477559b-7jj4r:No identity issued (No identity issued);mtls-deny:http-500;attestor-log:payments-service-778f6897f9-ltn2q:No identity issued (No identity issued);mtls-deny:http-500;attestor-log:payments-service-85c67b97dd-bdcbd:No identity issued (No identity issued);mtls-deny:http-500;env-vars-detected:4/4 |
| level2-attach-socket | PASS | PASS | attestor-log:payments-service-7d54995784-tlchw:No identity issued (No identity issued);svid-denied:payments-service-7d54995784-tlchw |
| level3-jar-unknown | PASS | PASS | attestor-log:payments-service-68c7c89779-4fx5s:No identity issued (No identity issued);mtls-deny:http-500 |
| bypass-cp-classpath | PASS | FAIL | assertion failed; see logs in /mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/attack-tests-attestor/results/run-20261015-193026/bypass-cp-classpath |
| bypass-symlink | PASS | PASS | decoy-symlink-in-place;runtime-selector-logging-disabled;mtls-ok:http-200 |
| bypass-symlink | PASS | PASS | decoy-symlink-in-place;runtime-selector-logging-disabled;mtls-ok:http-200 |
