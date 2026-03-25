package security

import (
	"regexp"
	"strings"
)

// DetectorConfig configures the injection detector.
type DetectorConfig struct {
	// Mode is either "strict" or "permissive".
	Mode string
}

// DetectionResult holds the outcome of an injection scan.
type DetectionResult struct {
	Detected bool
	Pattern  string // name of the matched pattern, empty if none
}

type injectionPattern struct {
	name    string
	pattern *regexp.Regexp
}

// InjectionDetector scans inputs for known prompt-injection patterns.
type InjectionDetector struct {
	cfg      DetectorConfig
	patterns []*injectionPattern
}

// NewInjectionDetector creates a detector with pre-compiled patterns.
func NewInjectionDetector(cfg DetectorConfig) *InjectionDetector {
	raw := []struct {
		name  string
		regex string
	}{
		{"ignore_previous", `(?i)ignore\s+(all\s+)?previous\s+instructions`},
		{"repeat_above", `(?i)repeat\s+(everything|all|the\s+text)\s+(above|before)`},
		{"delimiter_injection", `\[(IDENTITY|KNOWLEDGE|QUERY|SYSTEM)\]`},
		{"jailbreak_dan", `(?i)you\s+are\s+now\s+\w+\s+and\s+have\s+no\s+restrictions`},
		{"system_prompt_leak", `(?i)(show|reveal|print|output|display)\s+(your|the)\s+(system\s+)?prompt`},
		{"role_override", `(?i)(act|behave|pretend)\s+as\s+if\s+you\s+(have\s+no|are\s+not\s+bound)`},
	}

	patterns := make([]*injectionPattern, 0, len(raw))
	for _, r := range raw {
		patterns = append(patterns, &injectionPattern{
			name:    r.name,
			pattern: regexp.MustCompile(r.regex),
		})
	}

	return &InjectionDetector{cfg: cfg, patterns: patterns}
}

// Check scans input against all patterns and returns the first match.
func (d *InjectionDetector) Check(input string) DetectionResult {
	for _, p := range d.patterns {
		if p.pattern.MatchString(input) {
			return DetectionResult{Detected: true, Pattern: p.name}
		}
	}
	return DetectionResult{}
}

// CheckIdentityValues checks each value in the identity map for newlines,
// delimiter strings, and injection patterns.
func (d *InjectionDetector) CheckIdentityValues(identity map[string]string) DetectionResult {
	for _, v := range identity {
		// Newline injection check.
		if strings.ContainsAny(v, "\n\r") {
			return DetectionResult{Detected: true, Pattern: "newline_injection"}
		}
		// Run all compiled patterns against the value.
		if result := d.Check(v); result.Detected {
			return result
		}
	}
	return DetectionResult{}
}

// IsStrict returns true when the detector is configured in strict mode.
func (d *InjectionDetector) IsStrict() bool {
	return d.cfg.Mode == "strict"
}
