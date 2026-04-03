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

	fmt.Fprintf(w, "\nEstimated monthly savings at %d sessions/day: $%.2f/mo\n\n",
		report.Config.SessionsPerDay,
		report.MonthlySavingsDollars,
	)
}

func percent(saved, original int) int {
	if original == 0 {
		return 0
	}
	return (saved * 100) / original
}
