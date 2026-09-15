package procfs

import "strings"

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
