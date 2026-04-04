package redundancy

import (
	"sort"
	"strings"
	"sync"
)

// Result describes the outcome of a redundancy check.
type Result struct {
	IsRedundant bool    `json:"isRedundant"`
	Kind        string  `json:"kind"`       // "exact", "normalized", "similar", or ""
	Similarity  float64 `json:"similarity"` // 0.0–1.0
}

// Checker detects redundant content at three levels: exact, normalized, and similar.
// It is safe for concurrent use.
type Checker struct {
	mu         sync.Mutex
	threshold  float64
	raw        []string
	normalized []string
}

// NewChecker creates a Checker with the given Jaccard similarity threshold.
func NewChecker(threshold float64) *Checker {
	return &Checker{threshold: threshold}
}

// normalize sorts whitespace-separated tokens so order-independent comparison works.
func normalize(s string) string {
	tokens := strings.Fields(s)
	sort.Strings(tokens)
	return strings.Join(tokens, " ")
}

// Record stores content so future calls to Check can detect redundancy against it.
func (c *Checker) Record(content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.raw = append(c.raw, content)
	c.normalized = append(c.normalized, normalize(content))
}

// Check tests content against all previously recorded strings.
// It does NOT record the content — call Record separately if desired.
func (c *Checker) Check(content string) Result {
	c.mu.Lock()
	defer c.mu.Unlock()

	norm := normalize(content)
	normTokens := strings.Fields(norm)

	for i, r := range c.raw {
		if r == content {
			return Result{IsRedundant: true, Kind: "exact", Similarity: 1.0}
		}
		if c.normalized[i] == norm {
			return Result{IsRedundant: true, Kind: "normalized", Similarity: 1.0}
		}
		sim := jaccard(normTokens, strings.Fields(c.normalized[i]))
		if sim >= c.threshold {
			return Result{IsRedundant: true, Kind: "similar", Similarity: sim}
		}
	}
	return Result{}
}

func jaccard(a, b []string) float64 {
	setA := make(map[string]bool, len(a))
	for _, v := range a {
		setA[v] = true
	}
	union := make(map[string]bool, len(a)+len(b))
	for _, v := range a {
		union[v] = true
	}
	for _, v := range b {
		union[v] = true
	}
	intersection := 0
	for _, v := range b {
		if setA[v] {
			intersection++
		}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}
