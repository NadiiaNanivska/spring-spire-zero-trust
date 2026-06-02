package procfs

import "strings"

// SanitizeSelector replaces ':', '=', ' ', and control chars with '_' for safe use in SPIRE selector values.
func SanitizeSelector(s string) string {
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

// ParseEnviron parses the NUL-separated KEY=VALUE content of /proc/<PID>/environ into a map.
func ParseEnviron(raw string) map[string]string {
	entries := strings.Split(raw, "\x00")
	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		idx := strings.IndexByte(entry, '=')
		if idx < 0 {
			result[entry] = ""
			continue
		}
		result[entry[:idx]] = entry[idx+1:]
	}
	return result
}
