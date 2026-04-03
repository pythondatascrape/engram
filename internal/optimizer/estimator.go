package optimizer

import "math"

// Priority indicates how impactful an optimization is.
type Priority string

const (
	PriorityHigh   Priority = "HIGH"
	PriorityMedium Priority = "MEDIUM"
	PriorityLow    Priority = "LOW"
)

// EstimatorConfig holds tunable parameters for savings estimation.
type EstimatorConfig struct {
	SessionsPerDay          int
	TurnsPerSession         int
	CostPerMillionInput     float64
	CostPerMillionOutput    float64
	IdentityCompressionRate float64
	HistoryCompressionRate  float64
	ResponseCompressionRate float64
	AvgResponseTokens       int
}

// DefaultEstimatorConfig returns reasonable defaults for Claude usage.
func DefaultEstimatorConfig() EstimatorConfig {
	return EstimatorConfig{
		SessionsPerDay:          50,
		TurnsPerSession:         10,
		CostPerMillionInput:     3.0,
		CostPerMillionOutput:    15.0,
		IdentityCompressionRate: 0.96,
		HistoryCompressionRate:  0.80,
		ResponseCompressionRate: 0.20,
		AvgResponseTokens:       200,
	}
}

// SavingsItem is one line in the prioritized report.
type SavingsItem struct {
	Name             string
	OriginalTokens   int
	CompressedTokens int
	SavedTokens      int
	Priority         Priority
	Description      string
}

// SavingsReport is the full output of the estimator.
type SavingsReport struct {
	Items                   []SavingsItem
	TotalTokensSavedPerTurn int
	MonthlySavingsDollars   float64
	Config                  EstimatorConfig
}

// EstimateSavings calculates per-layer compression savings and dollar amounts.
func EstimateSavings(profile *ProjectProfile, cfg EstimatorConfig) *SavingsReport {
	report := &SavingsReport{Config: cfg}

	identityOriginal := profile.TotalTokens
	identitySaved := int(float64(identityOriginal) * cfg.IdentityCompressionRate)
	identityCompressed := identityOriginal - identitySaved
	report.Items = append(report.Items, SavingsItem{
		Name:             "Identity compression",
		OriginalTokens:   identityOriginal,
		CompressedTokens: identityCompressed,
		SavedTokens:      identitySaved,
		Priority:         rankPriority(identitySaved),
		Description:      "CLAUDE.md and identity files compressed to key=value format",
	})

	avgTurnTokens := 200
	avgPriorTurns := cfg.TurnsPerSession / 2
	historySavedPerTurn := int(float64(avgPriorTurns*avgTurnTokens) * cfg.HistoryCompressionRate)
	report.Items = append(report.Items, SavingsItem{
		Name:             "History compression",
		OriginalTokens:   avgPriorTurns * avgTurnTokens,
		CompressedTokens: avgPriorTurns*avgTurnTokens - historySavedPerTurn,
		SavedTokens:      historySavedPerTurn,
		Priority:         rankPriority(historySavedPerTurn),
		Description:      "Conversation history compressed with context codebook",
	})

	responseSaved := int(float64(cfg.AvgResponseTokens) * cfg.ResponseCompressionRate)
	report.Items = append(report.Items, SavingsItem{
		Name:             "Response metadata",
		OriginalTokens:   cfg.AvgResponseTokens,
		CompressedTokens: cfg.AvgResponseTokens - responseSaved,
		SavedTokens:      responseSaved,
		Priority:         rankPriority(responseSaved),
		Description:      "LLM response metadata (stop_reason, model, usage) compressed",
	})

	report.TotalTokensSavedPerTurn = identitySaved + historySavedPerTurn + responseSaved

	tokensPerSession := report.TotalTokensSavedPerTurn * cfg.TurnsPerSession
	tokensPerMonth := float64(tokensPerSession) * float64(cfg.SessionsPerDay) * 30.0
	report.MonthlySavingsDollars = math.Round(tokensPerMonth/1_000_000*cfg.CostPerMillionInput*100) / 100

	return report
}

func rankPriority(saved int) Priority {
	switch {
	case saved >= 1000:
		return PriorityHigh
	case saved >= 100:
		return PriorityMedium
	default:
		return PriorityLow
	}
}
