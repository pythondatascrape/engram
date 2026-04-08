package smc_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
)

func TestKController_EffectiveK(t *testing.T) {
	kc := smc.KController{
		Global:      0.5,
		PerCategory: map[string]float64{"entities": 0.3},
	}

	assert.Equal(t, 0.5, kc.EffectiveK("intent"))
	assert.Equal(t, 0.3, kc.EffectiveK("entities"))
	assert.Equal(t, 0.5, kc.EffectiveK("unknown"))
}

func TestKController_EffectiveK_NegativeOverrideUsesGlobal(t *testing.T) {
	kc := smc.KController{
		Global:      0.5,
		PerCategory: map[string]float64{"intent": -1},
	}
	assert.Equal(t, 0.5, kc.EffectiveK("intent"))
}

func TestKController_CompressionRatio(t *testing.T) {
	kc := smc.KController{Global: 0.5}

	// compression_ratio = 1 - (1 - min_ratio) * k
	// With min_ratio=0.1, k=0.5: 1 - (1-0.1)*0.5 = 1 - 0.45 = 0.55
	ratio := kc.CompressionRatio("intent", 0.1)
	assert.InDelta(t, 0.55, ratio, 0.001)
}

func TestKController_CompressionRatio_Extremes(t *testing.T) {
	tests := []struct {
		name     string
		k        float64
		minRatio float64
		want     float64
	}{
		{"k=0 maximum compression", 0.0, 0.1, 1.0},
		{"k=1 minimum compression", 1.0, 0.1, 0.1},
		{"k=0.5 mid", 0.5, 0.0, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := smc.KController{Global: tt.k}
			got := kc.CompressionRatio("any", tt.minRatio)
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

func TestNewKController(t *testing.T) {
	schema := smc.DefaultSchema()
	// Override entities k in schema
	schema.Categories[1].K = 0.3

	kc := smc.NewKController(0.5, schema)
	assert.Equal(t, 0.5, kc.Global)
	assert.Equal(t, 0.3, kc.PerCategory["entities"])
	// Categories with K=-1 should not appear in PerCategory
	_, hasIntent := kc.PerCategory["intent"]
	assert.False(t, hasIntent)
}
