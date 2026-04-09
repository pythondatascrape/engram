package smc

// KController manages the k-parameter for compression-fidelity tradeoff.
type KController struct {
	Global      float64            `yaml:"k"`
	PerCategory map[string]float64 `yaml:"-"` // built from schema
	AutoK       bool               `yaml:"auto_k"`
}

// NewKController creates a KController from a global k value and a schema.
// Per-category k values from the schema override the global when >= 0.
func NewKController(global float64, schema CategorySchema) KController {
	perCat := make(map[string]float64)
	for _, c := range schema.Categories {
		if c.K >= 0 {
			perCat[c.Name] = c.K
		}
	}
	return KController{
		Global:      global,
		PerCategory: perCat,
	}
}

// EffectiveK returns the k value for a category.
// Uses the per-category override if set and >= 0, otherwise the global value.
func (kc KController) EffectiveK(category string) float64 {
	if pk, ok := kc.PerCategory[category]; ok && pk >= 0 {
		return pk
	}
	return kc.Global
}

// CompressionRatio returns the target compression ratio for a category.
// Formula: compression_ratio = 1 - (1 - minRatio) * k
// At k=0: returns 1.0 (maximum compression).
// At k=1: returns minRatio (minimum compression, maximum fidelity).
func (kc KController) CompressionRatio(category string, minRatio float64) float64 {
	k := kc.EffectiveK(category)
	return 1 - (1-minRatio)*k
}
