// Package serializer converts validated identity maps to a compact self-describing format.
// Example: {"domain": "fire", "rank": "captain"} → "domain=fire rank=captain"
package serializer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
)

// Serializer converts identity maps to the Engram compact format.
type Serializer struct{}

// New returns a new Serializer.
func New() *Serializer {
	return &Serializer{}
}

// Serialize validates the identity map against the codebook and returns a
// deterministic key=value string sorted alphabetically by key.
func (s *Serializer) Serialize(cb *codebook.Codebook, identity map[string]string) (string, error) {
	if err := cb.Validate(identity); err != nil {
		return "", fmt.Errorf("identity validation failed: %w", err)
	}

	keys := make([]string, 0, len(identity))
	for k := range identity {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+identity[k])
	}

	return strings.Join(pairs, " "), nil
}
