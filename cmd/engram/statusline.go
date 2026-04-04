package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pythondatascrape/engram/internal/optimizer"
	"github.com/spf13/cobra"
)

func newStatuslineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "statusline [directory]",
		Short: "Print a compact token-savings chart for editor status lines",
		Long: `Scans a project directory for identity files and prints a three-row
bar chart (original / compressed / saved) suitable for embedding in
editor status lines such as Claude Code's statusLine setting.

When the Engram daemon is running, all three rows reflect actual runtime
token accounting from live sessions. Otherwise, static estimates are shown.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			profile, err := optimizer.ScanProject(absDir)
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			cfg := optimizer.DefaultEstimatorConfig()
			report := optimizer.EstimateSavings(profile, cfg)

			// Start with static estimates.
			data := optimizer.StatuslineEstimate(report)

			// Override with live stats from ~/.engram/stats.json if available.
			home, _ := os.UserHomeDir()
			statsPath := filepath.Join(home, ".engram", "stats.json")
			if raw, err := os.ReadFile(statsPath); err == nil {
				var s struct {
					TotalOriginalTokens   int `json:"totalOriginalTokens"`
					TotalCompressedTokens int `json:"totalCompressedTokens"`
					TotalSaved            int `json:"totalSaved"`
				}
				if json.Unmarshal(raw, &s) == nil && s.TotalOriginalTokens > 0 {
					data = optimizer.StatuslineData{
						Orig:  s.TotalOriginalTokens,
						Comp:  s.TotalCompressedTokens,
						Saved: s.TotalSaved,
						Live:  true,
					}
				}
			}

			optimizer.FormatStatusline(os.Stdout, data)
			return nil
		},
	}
	return cmd
}
