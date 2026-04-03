package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pythondatascrape/engram/internal/optimizer"
	"github.com/spf13/cobra"
)

func newAnalyzeCmd() *cobra.Command {
	var (
		sessionsPerDay int
		costPerMillion float64
		autoAccept     bool
	)

	cmd := &cobra.Command{
		Use:   "analyze [directory]",
		Short: "Analyze a project and show compression savings estimates",
		Long: `Scans a project directory for identity files (CLAUDE.md, AGENTS.md, etc.),
estimates token savings from Engram compression, and presents a prioritized
report with monthly dollar savings.`,
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

			projectName := filepath.Base(absDir)

			profile, err := optimizer.ScanProject(absDir)
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			cfg := optimizer.DefaultEstimatorConfig()
			if sessionsPerDay > 0 {
				cfg.SessionsPerDay = sessionsPerDay
			}
			if costPerMillion > 0 {
				cfg.CostPerMillionInput = costPerMillion
			}

			report := optimizer.EstimateSavings(profile, cfg)
			optimizer.FormatReportWithFiles(os.Stdout, projectName, report, profile.IdentityFiles)

			if len(profile.IdentityFiles) == 0 {
				return nil
			}

			if autoAccept {
				fmt.Println("Auto-accepting all optimizations.")
				return nil
			}

			fmt.Print("Accept all? [Y/n/pick] ")
			var answer string
			fmt.Scanln(&answer)
			switch answer {
			case "", "Y", "y", "yes":
				fmt.Println("All optimizations accepted. Run `engram serve` to start compressing.")
			case "n", "N", "no":
				fmt.Println("No changes made.")
			case "pick", "p":
				fmt.Println("Interactive selection not yet implemented. Use --yes to accept all.")
			default:
				fmt.Println("Unknown response. No changes made.")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&sessionsPerDay, "sessions", 0, "Override sessions per day (default: 50)")
	cmd.Flags().Float64Var(&costPerMillion, "cost", 0, "Override cost per million input tokens (default: $3.00)")
	cmd.Flags().BoolVarP(&autoAccept, "yes", "y", false, "Auto-accept all optimizations")

	return cmd
}
