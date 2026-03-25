package serializer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
)

func testCodebook(t *testing.T) *codebook.Codebook {
	t.Helper()
	cb, err := codebook.Parse([]byte(`
name: test
version: 1
dimensions:
  - name: domain
    type: enum
    required: true
    values: [fire, police, ems]
  - name: rank
    type: enum
    required: false
    values: [captain, lieutenant, sergeant]
  - name: experience
    type: range
    required: false
    min: 0
    max: 40
`))
	require.NoError(t, err)
	return cb
}

func TestSerialize_Success(t *testing.T) {
	s := serializer.New()
	cb := testCodebook(t)

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
	cb := testCodebook(t)

	identity := map[string]string{
		"domain": "invalid_domain",
		"rank":   "captain",
	}

	_, err := s.Serialize(cb, identity)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestSerialize_Deterministic(t *testing.T) {
	s := serializer.New()
	cb := testCodebook(t)

	identity := map[string]string{
		"domain":     "fire",
		"rank":       "lieutenant",
		"experience": "5",
	}

	result1, err1 := s.Serialize(cb, identity)
	result2, err2 := s.Serialize(cb, identity)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, result1, result2)
}
