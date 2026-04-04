package codebook

import (
	"sort"
	"strings"
)

// Derive parses key=value and "key: value" pairs from content and returns
// a map of the extracted dimensions and a deterministic serialized key=value string.
func Derive(content string) (map[string]string, string) {
	dims := make(map[string]string)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Try "key: value" style first (only single token key)
		if idx := strings.Index(line, ": "); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+2:])
			if isSimpleKey(key) && val != "" {
				dims[key] = val
				continue
			}
		}
		// Try space-separated key=value tokens within the line
		for _, token := range strings.Fields(line) {
			if idx := strings.Index(token, "="); idx > 0 {
				key := token[:idx]
				val := token[idx+1:]
				if key != "" && val != "" {
					dims[key] = val
				}
			}
		}
	}

	// Serialize deterministically
	keys := make([]string, 0, len(dims))
	for k := range dims {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + dims[k]
	}
	return dims, strings.Join(parts, " ")
}

// isSimpleKey returns true if s looks like a plain identifier (no spaces, no special chars).
func isSimpleKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '=' || r == ':' {
			return false
		}
	}
	return true
}
