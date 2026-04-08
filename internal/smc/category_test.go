package smc_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSchema(t *testing.T) {
	schema := smc.DefaultSchema()
	require.Len(t, schema.Categories, 4)

	names := make([]string, len(schema.Categories))
	for i, c := range schema.Categories {
		names[i] = c.Name
	}
	assert.Equal(t, []string{"intent", "entities", "mutations", "context"}, names)

	// All default categories use global k (indicated by -1)
	for _, c := range schema.Categories {
		assert.Equal(t, -1.0, c.K, "category %s should use global k", c.Name)
	}

	assert.True(t, schema.CrossRefs)
}

func TestCategorySchema_Validate(t *testing.T) {
	tests := []struct {
		name    string
		schema  smc.CategorySchema
		wantErr string
	}{
		{
			name:    "empty categories",
			schema:  smc.CategorySchema{},
			wantErr: "at least one category",
		},
		{
			name: "duplicate names",
			schema: smc.CategorySchema{
				Categories: []smc.Category{
					{Name: "intent", Description: "a"},
					{Name: "intent", Description: "b"},
				},
			},
			wantErr: "duplicate category",
		},
		{
			name: "blank name",
			schema: smc.CategorySchema{
				Categories: []smc.Category{
					{Name: "", Description: "a"},
				},
			},
			wantErr: "name must not be empty",
		},
		{
			name: "valid custom schema",
			schema: smc.CategorySchema{
				Categories: []smc.Category{
					{Name: "diagnosis", Description: "medical diagnosis", K: 0.3},
					{Name: "treatment", Description: "treatment plan", K: 0.7},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
