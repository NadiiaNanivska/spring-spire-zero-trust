# JVM Attestor Attack Test Summary

Run: `20260729-164730`

| Test | Expected | Status | Evidence |
|------|----------|--------|----------|
| bypass-cp-classpath | PASS | PASS | jvm-plugin-active;no-jvm-selectors:payments-service-85d54dc6d4-xhnnw;no-identity-issued:pid-46554;orders->payments:http-500 |

## Totals

- Tests run: 1
- Failures: 0

Status legend: **PASS** = defense worked; **FAIL** = defense missed; **LIMITATION (expected)** = documented bypass boundary.
