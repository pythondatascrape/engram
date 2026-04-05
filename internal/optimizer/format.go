package optimizer

import (
	"fmt"
	"io"
	"strings"
)

// FormatReport writes a human-readable prioritized savings report to w.
func FormatReport(w io.Writer, projectName string, report *SavingsReport) {
	FormatReportWithFiles(w, projectName, report, nil)
}

// FormatReportWithFiles writes the report with discovered file details.
func FormatReportWithFiles(w io.Writer, projectName string, report *SavingsReport, files []IdentityFile) {
	fmt.Fprintf(w, "\nEngram Project Analysis: %s\n", projectName)
	fmt.Fprintln(w, strings.Repeat("\u2501", 50))

	if len(files) > 0 {
		fmt.Fprintln(w, "\nDiscovered identity files:")
		for _, f := range files {
			fmt.Fprintf(w, "  - %s (%d tokens)\n", f.Name, f.TokenCount)
		}
	}

	if len(report.Items) == 0 {
		fmt.Fprintln(w, "\nNo identity files found. Nothing to optimize.")
		return
	}

	fmt.Fprintln(w, "\nPriority Optimizations:")
	for i, item := range report.Items {
		fmt.Fprintf(w, "  %d. %-25s \u2192 %d \u2192 ~%d tokens (%d%% savings)  [%s]\n",
			i+1,
			item.Name,
			item.OriginalTokens,
			item.CompressedTokens,
			percent(item.SavedTokens, item.OriginalTokens),
			item.Priority,
		)
	}

	// Token savings visualization
	totalOrig := 0
	totalComp := 0
	for _, item := range report.Items {
		totalOrig += item.OriginalTokens
		totalComp += item.CompressedTokens
	}
	totalSaved := totalOrig - totalComp
	savePct := percent(totalSaved, totalOrig)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Token Savings:")
	fmt.Fprintf(w, "  Original  %s %d tokens\n", bar(totalOrig, totalOrig, 30), totalOrig)
	fmt.Fprintf(w, "  Compressed%s %d tokens\n", bar(totalComp, totalOrig, 30), totalComp)
	fmt.Fprintf(w, "  Saved     %s %d tokens (%d%%)\n", bar(totalSaved, totalOrig, 30), totalSaved, savePct)

	fmt.Fprintf(w, "\nEstimated monthly savings at %d sessions/day: $%.2f/mo\n\n",
		report.Config.SessionsPerDay,
		report.MonthlySavingsDollars,
	)
}

// StatuslineData holds the values used to render the statusline chart.
// When the daemon is running, Orig/Comp/Saved reflect actual runtime token
// accounting. When the daemon is unavailable, they are static estimates.
type StatuslineData struct {
	Orig  int // tokens that would have been sent without Engram
	Comp  int // tokens actually sent
	Saved int // tokens saved
	Live  bool
}

// StatuslineEstimate builds StatuslineData from static analysis alone.
func StatuslineEstimate(report *SavingsReport) StatuslineData {
	orig, comp := 0, 0
	for _, item := range report.Items {
		orig += item.OriginalTokens
		comp += item.CompressedTokens
	}
	return StatuslineData{Orig: orig, Comp: comp, Saved: orig - comp}
}

// FormatStatusline writes a compact three-row bar chart for use in editor
// status lines (e.g. Claude Code statusLine).
func FormatStatusline(w io.Writer, d StatuslineData) {
	orig := d.Orig
	if orig == 0 {
		orig = 1 // avoid divide-by-zero in bar()
	}
	savePct := percent(d.Saved, orig)
	fmt.Fprintf(w, "orig  %s %d\n", bar(d.Orig, orig, 20), d.Orig)
	fmt.Fprintf(w, "comp  %s %d\n", bar(d.Comp, orig, 20), d.Comp)
	fmt.Fprintf(w, "saved %s %d (%d%%)\n", bar(d.Saved, orig, 20), d.Saved, savePct)
}

// ContextData holds token counts for context window impact display.
type ContextData struct {
	Orig int // context tokens without Engram (actual + saved)
	Comp int // context tokens with Engram (actual input tokens)
}

// fmtK formats a token count as a plain number below 1000, or rounded K above.
func fmtK(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%dK", (n+500)/1000)
}

// FormatStatuslineSideBySide renders identity stats (left) and context stats
// (right) on the same three lines, separated by four spaces.
func FormatStatuslineSideBySide(w io.Writer, d StatuslineData, ctx ContextData) {
	orig := d.Orig
	if orig == 0 {
		orig = 1
	}
	idSavePct := percent(d.Saved, orig)

	ctxOrig := ctx.Orig
	if ctxOrig == 0 {
		ctxOrig = 1
	}
	ctxSaved := ctxOrig - ctx.Comp
	ctxSavePct := percent(ctxSaved, ctxOrig)

	// Row 1: orig
	fmt.Fprintf(w, "orig  %s %d    orig  %s %s\n",
		bar(d.Orig, orig, 20), d.Orig,
		bar(ctx.Orig, ctx.Orig, 20), fmtK(ctx.Orig),
	)
	// Row 2: comp
	fmt.Fprintf(w, "comp  %s %d    comp  %s %s\n",
		bar(d.Comp, orig, 20), d.Comp,
		bar(ctx.Comp, ctx.Orig, 20), fmtK(ctx.Comp),
	)
	// Row 3: saved — identity shows "N (N%)", context shows "N%" only
	fmt.Fprintf(w, "saved %s %d (%d%%)    saved %s %d%%\n",
		bar(d.Saved, orig, 20), d.Saved, idSavePct,
		bar(ctxSaved, ctxOrig, 20), ctxSavePct,
	)
}

func percent(saved, original int) int {
	if original == 0 {
		return 0
	}
	return (saved * 100) / original
}

// bar renders a horizontal bar proportional to value/max, width chars wide.
func bar(value, max, width int) string {
	if max == 0 {
		return " " + strings.Repeat("\u2591", width)
	}
	filled := (value * width) / max
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return " " + strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}
