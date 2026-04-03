package optimizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateSavings_HighPriorityIdentity(t *testing.T) {
	profile := &ProjectProfile{
		Type: ProjectTypeGo,
		IdentityFiles: []IdentityFile{
			{Name: "CLAUDE.md", TokenCount: 2847},
		},
		TotalTokens: 2847,
	}

	report := EstimateSavings(profile, DefaultEstimatorConfig())
	assert.Len(t, report.Items, 3)

	identity := report.Items[0]
	assert.Equal(t, "Identity compression", identity.Name)
	assert.Equal(t, PriorityHigh, identity.Priority)
	assert.Equal(t, 2847, identity.OriginalTokens)
	assert.Greater(t, identity.SavedTokens, 1000)

	assert.Greater(t, report.MonthlySavingsDollars, 0.0)
}

func TestEstimateSavings_SmallProjectIsLowPriority(t *testing.T) {
	profile := &ProjectProfile{
		Type: ProjectTypeGo,
		IdentityFiles: []IdentityFile{
			{Name: "CLAUDE.md", TokenCount: 20},
		},
		TotalTokens: 20,
	}

	report := EstimateSavings(profile, DefaultEstimatorConfig())
	identity := report.Items[0]
	assert.Equal(t, PriorityLow, identity.Priority)
}

func TestEstimateSavings_NoIdentityFiles(t *testing.T) {
	profile := &ProjectProfile{
		Type:          ProjectTypeGo,
		IdentityFiles: []IdentityFile{},
		TotalTokens:   0,
	}

	report := EstimateSavings(profile, DefaultEstimatorConfig())
	// Identity savings should be zero, but history/response savings still apply.
	assert.Equal(t, 0, report.Items[0].SavedTokens)
	assert.Greater(t, report.MonthlySavingsDollars, 0.0)
}

func TestEstimateSavings_CustomConfig(t *testing.T) {
	profile := &ProjectProfile{
		Type: ProjectTypeGo,
		IdentityFiles: []IdentityFile{
			{Name: "CLAUDE.md", TokenCount: 2000},
		},
		TotalTokens: 2000,
	}

	cfg := EstimatorConfig{
		SessionsPerDay:          100,
		TurnsPerSession:         15,
		CostPerMillionInput:     3.0,
		CostPerMillionOutput:    15.0,
		IdentityCompressionRate: 0.96,
		HistoryCompressionRate:  0.80,
		ResponseCompressionRate: 0.20,
		AvgResponseTokens:       200,
	}

	report := EstimateSavings(profile, cfg)
	assert.Greater(t, report.MonthlySavingsDollars, 0.0)
	assert.Equal(t, 100, report.Config.SessionsPerDay)
}

func TestPriorityRanking(t *testing.T) {
	tests := []struct {
		saved    int
		expected Priority
	}{
		{1500, PriorityHigh},
		{500, PriorityMedium},
		{50, PriorityLow},
		{0, PriorityLow},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, rankPriority(tt.saved), "saved=%d", tt.saved)
	}
}
