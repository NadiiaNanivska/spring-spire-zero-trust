package internal

import "strings"

// sanitizeSelector replaces characters that are illegal in SPIRE selector values.
//
// SPIRE selector format is "type:value". The colon is the type/value separator,
// so a colon inside the value would break parsing. Spaces and ASCII control
// characters are also disallowed by the SPIFFE spec.
//
// Replacements: ':', '=', ' ', and any rune < 0x20  → '_'
//
// Example:
//
//	sanitizeSelector("-javaagent:")   → "-javaagent_"
//	sanitizeSelector("-Xrunjdwp:")    → "-Xrunjdwp_"
func sanitizeSelector(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == ':' || r == '=' || r == ' ' || r < 0x20 {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// parseEnviron parses the raw content of /proc/<PID>/environ into a map.
//
// The kernel writes the process environment as NUL-separated KEY=VALUE pairs
// with no trailing newline, e.g.:
//
//	PATH=/usr/bin\x00JAVA_TOOL_OPTIONS=-javaagent:/x.jar\x00HOME=/root\x00
//
// Keys without a '=' are stored with an empty string value.
// Empty entries (trailing NUL) are silently skipped.
func parseEnviron(raw string) map[string]string {
	entries := strings.Split(raw, "\x00")
	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		idx := strings.IndexByte(entry, '=')
		if idx < 0 {
			// Key with no value — unusual but valid in some environments.
			result[entry] = ""
			continue
		}
		result[entry[:idx]] = entry[idx+1:]
	}
	return result
}
