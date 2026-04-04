package codebook_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/stretchr/testify/assert"
)

func TestDerive_KeyEqValue(t *testing.T) {
	dims, serialized := codebook.Derive("lang=go arch=modular_monolith db=postgresql")
	assert.Equal(t, "go", dims["lang"])
	assert.Equal(t, "modular_monolith", dims["arch"])
	assert.Equal(t, "postgresql", dims["db"])
	assert.Contains(t, serialized, "lang=go")
	assert.Contains(t, serialized, "arch=modular_monolith")
}

func TestDerive_KeyColonValue(t *testing.T) {
	dims, serialized := codebook.Derive("lang: go\ndb: postgresql")
	assert.Equal(t, "go", dims["lang"])
	assert.Equal(t, "postgresql", dims["db"])
	assert.Contains(t, serialized, "lang=go")
}

func TestDerive_Empty(t *testing.T) {
	dims, serialized := codebook.Derive("")
	assert.Empty(t, dims)
	assert.Equal(t, "", serialized)
}

func TestDerive_SerializedIsDeterministic(t *testing.T) {
	_, s1 := codebook.Derive("b=2 a=1 c=3")
	_, s2 := codebook.Derive("c=3 b=2 a=1")
	assert.Equal(t, s1, s2)
	assert.Equal(t, "a=1 b=2 c=3", s1)
}
