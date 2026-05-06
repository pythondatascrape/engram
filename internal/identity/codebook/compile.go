package codebook

import (
	"fmt"
	"strings"
)

// Compile scans prose for high-frequency multi-syllable tokens (len > 6, freq > 1)
// and returns a substitution map of original_token → short_abbreviation.
// The abbreviations are unique within the returned map.
func Compile(text string) (map[string]string, error) {
	if text == "" {
		return map[string]string{}, nil
	}

	// Count normalized (lowercased) token frequencies.
	freq := make(map[string]int)
	for _, tok := range strings.Fields(text) {
		tok = strings.ToLower(strings.Trim(tok, ".,;:!?\"'()[]{}"))
		if len(tok) > 6 {
			freq[tok]++
		}
	}

	// Collect candidates: tokens that appear more than once.
	var candidates []string
	for tok, count := range freq {
		if count > 1 {
			candidates = append(candidates, tok)
		}
	}

	if len(candidates) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string, len(candidates))
	usedAbbrs := make(map[string]struct{})

	for _, tok := range candidates {
		abbr, err := uniqueAbbreviation(tok, usedAbbrs)
		if err != nil {
			return nil, err
		}
		result[tok] = abbr
		usedAbbrs[abbr] = struct{}{}
	}

	return result, nil
}

// uniqueAbbreviation generates a short abbreviation for tok that does not collide
// with already-used abbreviations. It tries increasing prefix lengths starting at 3.
func uniqueAbbreviation(tok string, used map[string]struct{}) (string, error) {
	runes := []rune(tok)
	for length := 3; length < len(runes); length++ {
		candidate := string(runes[:length])
		if _, taken := used[candidate]; !taken {
			return candidate, nil
		}
	}
	// Fall back: append a numeric suffix to a 3-char prefix.
	base := string(runes[:3])
	for i := 2; i <= len(runes); i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, taken := used[candidate]; !taken {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not generate unique abbreviation for %q", tok)
}
