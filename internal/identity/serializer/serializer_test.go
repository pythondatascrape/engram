package serializer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
)

// newTestCodebook builds a codebook with known fields for testing.
func newTestCodebook() *codebook.Codebook {
	cb, _ := codebook.Parse(nil)
	cb.AddField(codebook.Field{Name: "domain", Type: "enum", Values: []string{"fire", "police", "ems"}})
	cb.AddField(codebook.Field{Name: "rank", Type: "enum", Values: []string{"captain", "lieutenant", "sergeant"}})
	cb.AddField(codebook.Field{Name: "experience", Type: "string"})
	return cb
}

func TestSerialize_Success(t *testing.T) {
	s := serializer.New()
	cb := newTestCodebook()

	identity := map[string]string{
		"domain":     "fire",
		"rank":       "captain",
		"experience": "20",
	}

	result, err := s.Serialize(cb, identity)
	require.NoError(t, err)
	assert.Equal(t, "domain=fire experience=20 rank=captain", result)
}

func TestSerialize_ValidationError(t *testing.T) {
	s := serializer.New()
	cb := newTestCodebook()

	identity := map[string]string{
		"domain": "invalid_domain", // not a valid enum value
		"rank":   "captain",
	}

	_, err := s.Serialize(cb, identity)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestSerialize_Deterministic(t *testing.T) {
	s := serializer.New()
	cb := newTestCodebook()

	identity := map[string]string{
		"domain":     "fire",
		"rank":       "lieutenant",
		"experience": "5",
	}

	result1, err1 := s.Serialize(cb, identity)
	result2, err2 := s.Serialize(cb, identity)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, result1, result2, "output must be deterministic across calls")
}
