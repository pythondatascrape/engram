package optimizer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatReport_ContainsAllSections(t *testing.T) {
	report := &SavingsReport{
		Items: []SavingsItem{
			{Name: "Identity compression", OriginalTokens: 2847, CompressedTokens: 120, SavedTokens: 2727, Priority: PriorityHigh},
			{Name: "History compression", OriginalTokens: 1000, CompressedTokens: 200, SavedTokens: 800, Priority: PriorityHigh},
			{Name: "Response metadata", OriginalTokens: 200, CompressedTokens: 160, SavedTokens: 40, Priority: PriorityMedium},
		},
		MonthlySavingsDollars: 842.50,
		Config:                DefaultEstimatorConfig(),
	}

	var buf bytes.Buffer
	FormatReport(&buf, "my-project", report)
	output := buf.String()

	assert.Contains(t, output, "my-project")
	assert.Contains(t, output, "Identity compression")
	assert.Contains(t, output, "History compression")
	assert.Contains(t, output, "Response metadata")
	assert.Contains(t, output, "HIGH")
	assert.Contains(t, output, "$842.50")
	assert.Contains(t, output, "50 sessions/day")
}

func TestFormatReport_EmptyReport(t *testing.T) {
	report := &SavingsReport{
		Items:  []SavingsItem{},
		Config: DefaultEstimatorConfig(),
	}

	var buf bytes.Buffer
	FormatReport(&buf, "empty-project", report)
	output := buf.String()

	assert.Contains(t, output, "No identity files found")
}

func TestFormatReport_ListsFoundFiles(t *testing.T) {
	report := &SavingsReport{
		Items: []SavingsItem{
			{Name: "Identity compression", OriginalTokens: 500, CompressedTokens: 25, SavedTokens: 475, Priority: PriorityMedium},
		},
		MonthlySavingsDollars: 10.0,
		Config:                DefaultEstimatorConfig(),
	}

	var buf bytes.Buffer
	files := []IdentityFile{
		{Name: "CLAUDE.md", TokenCount: 400},
		{Name: ".claude/CLAUDE.md", TokenCount: 100},
	}
	FormatReportWithFiles(&buf, "test-proj", report, files)
	output := buf.String()

	assert.True(t, strings.Contains(output, "CLAUDE.md"))
	assert.True(t, strings.Contains(output, "400 tokens"))
}
