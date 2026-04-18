package codebook

import (
	"regexp"
	"sort"
	"strings"
)

// proseRule maps a compiled regex to the dimension key and value it implies.
type proseRule struct {
	re  *regexp.Regexp
	key string
	val string
}

var proseRules = []proseRule{
	{re: regexp.MustCompile(`(?i)prefer concise|concise responses?|be concise`), key: "response_style", val: "concise"},
	{re: regexp.MustCompile(`(?i)no trailing summar|do not.*summar|don't.*summar`), key: "summary_policy", val: "no_trailing_summary"},
	{re: regexp.MustCompile(`(?i)senior\s+\w*\s*engineer|I am a\s+\w*\s*engineer|I['']m a\s+\w*\s*engineer`), key: "seniority", val: "senior"},
	{re: regexp.MustCompile(`(?i)software engineer|backend engineer|frontend engineer|full.?stack engineer`), key: "role", val: "engineer"},
	{re: regexp.MustCompile(`(?i)target macOS.*linux|target linux.*macOS|macOS and linux|linux and macOS`), key: "platform", val: "macos,linux"},
	{re: regexp.MustCompile(`(?i)target macOS`), key: "platform", val: "macos"},
	{re: regexp.MustCompile(`(?i)target linux`), key: "platform", val: "linux"},
	{re: regexp.MustCompile(`(?i)\blang(?:uage)?[:\s]+go\b|written in go\b|using go\b`), key: "lang", val: "go"},
	{re: regexp.MustCompile(`(?i)\blang(?:uage)?[:\s]+python\b|written in python\b|using python\b`), key: "lang", val: "python"},
	{re: regexp.MustCompile(`(?i)\blang(?:uage)?[:\s]+typescript\b|written in typescript\b`), key: "lang", val: "typescript"},
	{re: regexp.MustCompile(`(?i)modular.monolith`), key: "arch", val: "modular_monolith"},
	{re: regexp.MustCompile(`(?i)microservices?\s+arch`), key: "arch", val: "microservices"},
}

// deriveProse scans free-form prose for well-known patterns and returns a map of dimensions.
func deriveProse(content string) map[string]string {
	dims := make(map[string]string)
	for _, rule := range proseRules {
		if _, already := dims[rule.key]; already {
			continue
		}
		if rule.re.MatchString(content) {
			dims[rule.key] = rule.val
		}
	}
	return dims
}

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

	// Merge prose-derived dims; key=value/key:value takes precedence.
	for k, v := range deriveProse(content) {
		if _, exists := dims[k]; !exists {
			dims[k] = v
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
