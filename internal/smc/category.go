package smc

import "fmt"

// Category defines a single decomposition dimension.
type Category struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	K           float64 `yaml:"k"` // per-category k; -1 means use global
}

// CategorySchema defines the set of categories used for turn decomposition.
type CategorySchema struct {
	Categories []Category `yaml:"categories"`
	CrossRefs  bool       `yaml:"cross_refs"`
}

// Validate checks that the schema has at least one category, no duplicates,
// and no blank names.
func (s CategorySchema) Validate() error {
	if len(s.Categories) == 0 {
		return fmt.Errorf("smc: schema must have at least one category")
	}
	seen := make(map[string]bool, len(s.Categories))
	for _, c := range s.Categories {
		if c.Name == "" {
			return fmt.Errorf("smc: category name must not be empty")
		}
		if seen[c.Name] {
			return fmt.Errorf("smc: duplicate category %q", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

// Names returns the ordered list of category names.
func (s CategorySchema) Names() []string {
	names := make([]string, len(s.Categories))
	for i, c := range s.Categories {
		names[i] = c.Name
	}
	return names
}

// DefaultSchema returns the default four-category schema for software engineering.
func DefaultSchema() CategorySchema {
	return CategorySchema{
		CrossRefs: true,
		Categories: []Category{
			{Name: "intent", Description: "What the speaker wants or is doing — semantic purpose, goals, directives", K: -1},
			{Name: "entities", Description: "Files, functions, concepts referenced — concrete nouns, identifiers, domain objects", K: -1},
			{Name: "mutations", Description: "What changed — code diffs, state transitions, decisions, commitments", K: -1},
			{Name: "context", Description: "Reasoning, constraints, rationale — the 'why' behind actions and decisions", K: -1},
		},
	}
}
