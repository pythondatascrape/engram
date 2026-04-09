// Package context provides context-turn compression for Engram sessions.
package context

import (
	"fmt"
	"sort"
	"strings"
)

// ContextCodebook holds a derived codebook for compressing conversation turns.
type ContextCodebook struct {
	Name     string
	keys     []string          // sorted field names from schema
	schema   map[string]string // field name → type hint (stored for Definition())
	defaults map[string]string // enum field → first value (the default)
}

// DeriveCodebook derives a ContextCodebook from a schema map.
// schema maps field names to type hints (e.g. "enum:user,assistant" or "text").
// Returns an error if the schema is empty or a field name is blank.
func DeriveCodebook(name string, schema map[string]string) (*ContextCodebook, error) {
	if len(schema) == 0 {
		return nil, fmt.Errorf("context codebook %q: schema must have at least one field", name)
	}
	keys := make([]string, 0, len(schema))
	for k := range schema {
		if k == "" {
			return nil, fmt.Errorf("context codebook %q: field name must not be empty", name)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	schemaCopy := make(map[string]string, len(schema))
	for k, v := range schema {
		schemaCopy[k] = v
	}
	defaults := make(map[string]string)
	for k, v := range schemaCopy {
		if strings.HasPrefix(v, "enum:") {
			values := strings.SplitN(strings.TrimPrefix(v, "enum:"), ",", 2)
			if len(values) > 0 && values[0] != "" {
				defaults[k] = values[0]
			}
		}
	}
	return &ContextCodebook{Name: name, keys: keys, schema: schemaCopy, defaults: defaults}, nil
}

// Defaults returns a copy of the enum default values map.
func (c *ContextCodebook) Defaults() map[string]string {
	out := make(map[string]string, len(c.defaults))
	for k, v := range c.defaults {
		out[k] = v
	}
	return out
}

// Keys returns the sorted field names in this codebook.
func (c *ContextCodebook) Keys() []string {
	out := make([]string, len(c.keys))
	copy(out, c.keys)
	return out
}

// SerializeTurn compresses a turn map to "key=value key=value" format.
// Returns an error if the turn contains fields not in the schema.
func (c *ContextCodebook) SerializeTurn(turn map[string]string) (string, error) {
	for k := range turn {
		if _, ok := c.schema[k]; !ok {
			return "", fmt.Errorf("context codebook %q: unknown field %q in turn", c.Name, k)
		}
	}

	var b strings.Builder
	first := true
	for _, k := range c.keys {
		v, ok := turn[k]
		if !ok {
			continue
		}
		if def, hasDef := c.defaults[k]; hasDef && v == def {
			continue
		}
		if !first {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
		first = false
	}
	return b.String(), nil
}

// Definition returns a compact human-readable codebook definition for injection
// into the system prompt so the LLM understands the compressed format.
// Format: "name: field1(type) field2(type) ..."
func (c *ContextCodebook) Definition() string {
	var b strings.Builder
	b.WriteString(c.Name)
	b.WriteString(": ")
	for i, k := range c.keys {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('(')

		typeHint := c.schema[k]
		if def, ok := c.defaults[k]; ok && strings.HasPrefix(typeHint, "enum:") {
			enumPart := strings.TrimPrefix(typeHint, "enum:")
			enumPart = strings.Replace(enumPart, def, def+"*", 1)
			b.WriteString("enum:")
			b.WriteString(enumPart)
		} else {
			b.WriteString(typeHint)
		}

		b.WriteByte(')')
	}
	return b.String()
}
