package optimizer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFmtK(t *testing.T) {
	assert.Equal(t, "0", fmtK(0))
	assert.Equal(t, "42", fmtK(42))
	assert.Equal(t, "999", fmtK(999))
	assert.Equal(t, "1K", fmtK(1000))
	assert.Equal(t, "1K", fmtK(1499))
	assert.Equal(t, "2K", fmtK(1500))
	assert.Equal(t, "44K", fmtK(44300))
	assert.Equal(t, "45K", fmtK(45000))
}

func TestFormatStatuslineSideBySide_RendersThreeRows(t *testing.T) {
	var buf bytes.Buffer
	d := StatuslineData{Orig: 525, Comp: 28, Saved: 497, Live: true}
	ctx := ContextData{Orig: 45000, Comp: 44499}
	FormatStatuslineSideBySide(&buf, d, ctx)
	out := buf.String()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "orig")
	assert.Contains(t, lines[0], "525")
	assert.Contains(t, lines[0], "45K")
	assert.Contains(t, lines[1], "comp")
	assert.Contains(t, lines[1], "28")
	assert.Contains(t, lines[1], "44K")
	assert.Contains(t, lines[2], "saved")
	assert.Contains(t, lines[2], "497")
	assert.Contains(t, lines[2], "%")
}

func TestFormatStatuslineSideBySide_ZeroSaved(t *testing.T) {
	var buf bytes.Buffer
	d := StatuslineData{Orig: 100, Comp: 100, Saved: 0, Live: true}
	ctx := ContextData{Orig: 5000, Comp: 5000}
	FormatStatuslineSideBySide(&buf, d, ctx)
	out := buf.String()
	assert.Contains(t, out, "0%")
}

func TestFormatStatuslineSideBySide_SubKContext(t *testing.T) {
	var buf bytes.Buffer
	d := StatuslineData{Orig: 75, Comp: 4, Saved: 71, Live: true}
	ctx := ContextData{Orig: 800, Comp: 729}
	FormatStatuslineSideBySide(&buf, d, ctx)
	out := buf.String()
	// Sub-1K values render as plain numbers, no K suffix
	assert.NotContains(t, out, "800K")
	assert.Contains(t, out, "800")
	assert.Contains(t, out, "729")
}

func TestFormatStatuslineSideBySide_UsesRawContextPercent(t *testing.T) {
	var buf bytes.Buffer
	d := StatuslineData{Orig: 75, Comp: 4, Saved: 71, Live: true}
	ctx := ContextData{Orig: 2000, Comp: 444}
	FormatStatuslineSideBySide(&buf, d, ctx)
	out := buf.String()

	// 2000 -> 2K and 444 -> 444, but the percentage must still be computed
	// from raw values: (2000-444)/2000 = 77% with integer truncation.
	assert.Contains(t, out, "2K")
	assert.Contains(t, out, "444")
	assert.Contains(t, out, "77%")
	assert.NotContains(t, out, "100%")
}

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
