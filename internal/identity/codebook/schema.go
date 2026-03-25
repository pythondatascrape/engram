// Package codebook provides parsing and validation of identity codebooks.
// This is a stub implementation — the full implementation will replace this.
package codebook

import "fmt"

// Field represents a single field definition in the codebook.
type Field struct {
	Name   string
	Type   string   // "enum" or "string"
	Values []string // valid values for enum fields
}

// Codebook holds the parsed schema for validating identity maps.
type Codebook struct {
	fields map[string]Field
}

// Parse parses YAML bytes into a Codebook.
func Parse(yamlBytes []byte) (*Codebook, error) {
	// Stub: returns empty codebook
	return &Codebook{fields: make(map[string]Field)}, nil
}

// Validate checks that the identity map conforms to the codebook schema.
func (cb *Codebook) Validate(identity map[string]string) error {
	for key, val := range identity {
		f, ok := cb.fields[key]
		if !ok {
			continue
		}
		if f.Type == "enum" {
			valid := false
			for _, v := range f.Values {
				if v == val {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("field %q: invalid enum value %q", key, val)
			}
		}
	}
	return nil
}

// AddField adds a field to the codebook (used in tests/stubs).
func (cb *Codebook) AddField(f Field) {
	cb.fields[f.Name] = f
}
