// Package codebook provides parsing and validation of identity codebooks.
package codebook

import (
	"fmt"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// DimType represents the type of a dimension.
type DimType string

const (
	DimEnum    DimType = "enum"
	DimRange   DimType = "range"
	DimScale   DimType = "scale"
	DimBoolean DimType = "boolean"
)

var validDimTypes = map[DimType]struct{}{
	DimEnum:    {},
	DimRange:   {},
	DimScale:   {},
	DimBoolean: {},
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

const (
	defaultMaxDimensions = 50
	defaultMaxEnumValues = 20
)

// Dimension describes a single dimension in a codebook.
type Dimension struct {
	Name        string   `yaml:"name"`
	Type        DimType  `yaml:"type"`
	Required    bool     `yaml:"required"`
	Values      []string `yaml:"values,omitempty"`
	Min         float64  `yaml:"min,omitempty"`
	Max         float64  `yaml:"max,omitempty"`
	Default     string   `yaml:"default,omitempty"`
	Description string   `yaml:"description,omitempty"`
}

// Codebook is the top-level schema defining identity dimensions.
type Codebook struct {
	Name       string      `yaml:"name"`
	Version    int         `yaml:"version"`
	Dimensions []Dimension `yaml:"dimensions"`
}

// Parse parses a codebook from YAML bytes using default limits (50 dims, 20 enum values).
func Parse(data []byte) (*Codebook, error) {
	return ParseWithLimits(data, defaultMaxDimensions, defaultMaxEnumValues)
}

// ParseWithLimits parses a codebook from YAML bytes with explicit dimension and enum value limits.
func ParseWithLimits(data []byte, maxDims, maxEnumValues int) (*Codebook, error) {
	var cb Codebook
	if err := yaml.Unmarshal(data, &cb); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}

	if !namePattern.MatchString(cb.Name) {
		return nil, fmt.Errorf("invalid codebook name %q: must match ^[a-z][a-z0-9_]{0,63}$", cb.Name)
	}

	if len(cb.Dimensions) > maxDims {
		return nil, fmt.Errorf("too many dimensions: %d exceeds limit of %d", len(cb.Dimensions), maxDims)
	}

	seen := make(map[string]struct{}, len(cb.Dimensions))
	for _, d := range cb.Dimensions {
		if !namePattern.MatchString(d.Name) {
			return nil, fmt.Errorf("invalid dimension name %q: must match ^[a-z][a-z0-9_]{0,63}$", d.Name)
		}

		if _, dup := seen[d.Name]; dup {
			return nil, fmt.Errorf("duplicate dimension name %q", d.Name)
		}
		seen[d.Name] = struct{}{}

		if _, ok := validDimTypes[d.Type]; !ok {
			return nil, fmt.Errorf("unknown dimension type %q for dimension %q", d.Type, d.Name)
		}

		switch d.Type {
		case DimEnum:
			if len(d.Values) == 0 {
				return nil, fmt.Errorf("enum dimension %q must have at least 1 value", d.Name)
			}
			if len(d.Values) > maxEnumValues {
				return nil, fmt.Errorf("enum dimension %q has %d values, exceeds limit of %d", d.Name, len(d.Values), maxEnumValues)
			}
		case DimRange, DimScale:
			if d.Min >= d.Max {
				return nil, fmt.Errorf("dimension %q: min (%v) must be less than max (%v)", d.Name, d.Min, d.Max)
			}
		}
	}

	return &cb, nil
}

// Validate checks that the provided identity map conforms to the codebook schema.
func (cb *Codebook) Validate(identity map[string]string) error {
	dimMap := make(map[string]*Dimension, len(cb.Dimensions))
	for i := range cb.Dimensions {
		dimMap[cb.Dimensions[i].Name] = &cb.Dimensions[i]
	}

	// Reject unknown dimensions.
	for key := range identity {
		if _, ok := dimMap[key]; !ok {
			return fmt.Errorf("unknown dimension %q", key)
		}
	}

	// Check required presence and validate values.
	for _, d := range cb.Dimensions {
		val, present := identity[d.Name]
		if !present {
			if d.Required {
				return fmt.Errorf("required dimension %q is missing", d.Name)
			}
			continue
		}

		switch d.Type {
		case DimEnum:
			if !contains(d.Values, val) {
				return fmt.Errorf("dimension %q: value %q is not one of %v", d.Name, val, d.Values)
			}
		case DimRange, DimScale:
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf("dimension %q: value %q is not a number", d.Name, val)
			}
			if f < d.Min || f > d.Max {
				return fmt.Errorf("dimension %q: value %v is out of range [%v, %v]", d.Name, f, d.Min, d.Max)
			}
		case DimBoolean:
			if val != "true" && val != "false" {
				return fmt.Errorf("dimension %q: value %q must be \"true\" or \"false\"", d.Name, val)
			}
		}
	}

	return nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
