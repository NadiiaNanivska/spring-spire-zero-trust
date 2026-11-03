# JVM Attestor Attack Test Summary

Run: `20261015-194958`

| Test | Expected | Status | Evidence |
|------|----------|--------|----------|
| bypass-cp-classpath | PASS | PASS | jvm-plugin-active;runtime-selector-logging-disabled;no-identity-issued:pid-24323;orders->payments:http-500 |
| bypass-mmap-shadow | PASS | PASS | mmap-shadow-in-place:1-maps-entries;jvm-plugin-active;runtime-selector-logging-disabled;no-identity-issued:pid-25623;orders->payments:http-500 |
| level3-provenance | PASS | PASS | attestor-log:payments-service-6d644b96d9-jjfdc:No identity issued (No identity issued);svid-denied:payments-service-6d644b96d9-jjfdc;provenance-corrupt-rejected;attestor-log:payments-service-6d644b96d9-kzhxj:No identity issued (No identity issued);svid-denied:payments-service-6d644b96d9-kzhxj;provenance-missing-rejected;attestor-log:payments-service-6d644b96d9-mdktr:No identity issued (No identity issued);svid-denied:payments-service-6d644b96d9-mdktr;provenance-symlink-rejected |

## Totals

- Tests run: 3
- Failures: 0

Status legend: **PASS** = defense worked; **FAIL** = defense missed; **LIMITATION (expected)** = documented bypass boundary.
