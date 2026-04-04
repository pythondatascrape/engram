package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pythondatascrape/engram/internal/daemon"
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

			// Override with live daemon stats if available.
			socketPath, _ := cmd.Flags().GetString("socket")
			if socketPath == "" {
				socketPath = defaultSocketPath()
			}
			if client, err := daemon.NewClient(socketPath); err == nil {
				defer client.Close()
				if stats, err := client.Stats(); err == nil && stats.CumulativeBaseline > 0 {
					data = optimizer.StatuslineData{
						Orig:  stats.CumulativeBaseline,
						Comp:  stats.TokensSent,
						Saved: stats.TotalSaved,
						Live:  true,
					}
				}
			}

			optimizer.FormatStatusline(os.Stdout, data)
			return nil
		},
	}
	cmd.Flags().String("socket", "", "Unix socket path (default: ~/.engram/engram.sock)")
	return cmd
}
