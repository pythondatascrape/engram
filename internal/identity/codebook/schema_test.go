package codebook_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validYAML = `
name: test_codebook
version: 1
dimensions:
  - name: role
    type: enum
    required: true
    values: [admin, user, guest]
    description: User role
  - name: age
    type: range
    required: false
    min: 0
    max: 120
    description: Age in years
  - name: score
    type: scale
    required: false
    min: 0
    max: 10
    default: "5"
  - name: active
    type: boolean
    required: true
    description: Is user active
`

func TestParse_ValidCodebook(t *testing.T) {
	cb, err := codebook.Parse([]byte(validYAML))
	require.NoError(t, err)
	assert.Equal(t, "test_codebook", cb.Name)
	assert.Equal(t, 1, cb.Version)
	assert.Len(t, cb.Dimensions, 4)
	assert.Equal(t, "role", cb.Dimensions[0].Name)
	assert.Equal(t, codebook.DimEnum, cb.Dimensions[0].Type)
	assert.True(t, cb.Dimensions[0].Required)
	assert.Equal(t, []string{"admin", "user", "guest"}, cb.Dimensions[0].Values)
}

func TestValidate_ValidIdentity(t *testing.T) {
	cb, err := codebook.Parse([]byte(validYAML))
	require.NoError(t, err)

	identity := map[string]string{
		"role":   "admin",
		"age":    "30",
		"score":  "7",
		"active": "true",
	}
	err = cb.Validate(identity)
	assert.NoError(t, err)
}

func TestValidate_MissingRequiredDimension(t *testing.T) {
	cb, err := codebook.Parse([]byte(validYAML))
	require.NoError(t, err)

	identity := map[string]string{
		// "role" is missing — required
		"active": "true",
	}
	err = cb.Validate(identity)
	assert.ErrorContains(t, err, "role")
}

func TestValidate_InvalidEnumValue(t *testing.T) {
	cb, err := codebook.Parse([]byte(validYAML))
	require.NoError(t, err)

	identity := map[string]string{
		"role":   "superuser",
		"active": "true",
	}
	err = cb.Validate(identity)
	assert.ErrorContains(t, err, "role")
}

func TestValidate_OutOfRangeValue(t *testing.T) {
	cb, err := codebook.Parse([]byte(validYAML))
	require.NoError(t, err)

	identity := map[string]string{
		"role":   "admin",
		"age":    "200",
		"active": "true",
	}
	err = cb.Validate(identity)
	assert.ErrorContains(t, err, "age")
}

func TestValidate_UnknownDimension(t *testing.T) {
	cb, err := codebook.Parse([]byte(validYAML))
	require.NoError(t, err)

	identity := map[string]string{
		"role":    "admin",
		"active":  "true",
		"unknown": "value",
	}
	err = cb.Validate(identity)
	assert.ErrorContains(t, err, "unknown")
}

func TestParse_InvalidDimensionName(t *testing.T) {
	data := `
name: test_codebook
version: 1
dimensions:
  - name: Invalid-Name
    type: enum
    values: [a, b]
`
	_, err := codebook.Parse([]byte(data))
	assert.ErrorContains(t, err, "Invalid-Name")
}

func TestParse_TooManyDimensions(t *testing.T) {
	// Build a codebook with 51 dimensions using ParseWithLimits at max=50
	dims := "name: big\nversion: 1\ndimensions:\n"
	for i := 0; i < 51; i++ {
		dims += "  - name: dim_" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "\n    type: boolean\n    required: false\n"
	}
	_, err := codebook.ParseWithLimits([]byte(dims), 50, 20)
	assert.ErrorContains(t, err, "dimensions")
}

func TestParse_InvalidCodebookName(t *testing.T) {
	data := `
name: Bad-Name
version: 1
dimensions: []
`
	_, err := codebook.Parse([]byte(data))
	assert.ErrorContains(t, err, "Bad-Name")
}

func TestParse_DuplicateDimensionNames(t *testing.T) {
	data := `
name: test
version: 1
dimensions:
  - name: role
    type: enum
    values: [a]
  - name: role
    type: boolean
`
	_, err := codebook.Parse([]byte(data))
	assert.ErrorContains(t, err, "role")
}

func TestParse_EnumWithNoValues(t *testing.T) {
	data := `
name: test
version: 1
dimensions:
  - name: role
    type: enum
    values: []
`
	_, err := codebook.Parse([]byte(data))
	assert.Error(t, err)
}

func TestParse_RangeMinNotLessThanMax(t *testing.T) {
	data := `
name: test
version: 1
dimensions:
  - name: score
    type: range
    min: 10
    max: 5
`
	_, err := codebook.Parse([]byte(data))
	assert.ErrorContains(t, err, "score")
}

func TestParse_UnknownDimensionType(t *testing.T) {
	data := `
name: test
version: 1
dimensions:
  - name: score
    type: freetext
`
	_, err := codebook.Parse([]byte(data))
	assert.ErrorContains(t, err, "freetext")
}

func TestValidate_InvalidBooleanValue(t *testing.T) {
	cb, err := codebook.Parse([]byte(validYAML))
	require.NoError(t, err)

	identity := map[string]string{
		"role":   "admin",
		"active": "yes",
	}
	err = cb.Validate(identity)
	assert.ErrorContains(t, err, "active")
}

func TestParseWithLimits_InvalidYAML(t *testing.T) {
	_, err := codebook.ParseWithLimits([]byte(`{{{not yaml`), 50, 20)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "yaml parse error")
}

func TestParseWithLimits_EnumValuesExceedLimit(t *testing.T) {
	yml := `
name: test
version: 1
dimensions:
  - name: color
    type: enum
    required: false
    values: [red, green, blue, yellow, cyan]
`
	_, err := codebook.ParseWithLimits([]byte(yml), 50, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func TestValidate_RangeValueNotANumber(t *testing.T) {
	yml := `
name: test
version: 1
dimensions:
  - name: score
    type: range
    required: true
    min: 0
    max: 100
`
	cb, err := codebook.Parse([]byte(yml))
	require.NoError(t, err)

	identity := map[string]string{
		"score": "not-a-number",
	}
	err = cb.Validate(identity)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a number")
}
