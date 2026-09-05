# JVM Attestor Attack Test Summary

Run: `20260901-212356`

| Test | Expected | Status | Evidence |
|------|----------|--------|----------|
| bypass-mmap-shadow | PASS | PASS | mmap-shadow-in-place:1-maps-entries;jvm-plugin-active;discovery:maps+fd-unioned;approved-jar-selector-still-present;jar-set-digest-changed;no-identity-issued:pid-37535;orders->payments:http-500 |

## Totals

- Tests run: 1
- Failures: 0

Status legend: **PASS** = defense worked; **FAIL** = defense missed; **LIMITATION (expected)** = documented bypass boundary.
