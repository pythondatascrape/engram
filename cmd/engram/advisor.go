package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pythondatascrape/engram/internal/optimizer"
	"github.com/spf13/cobra"
)

func newAdvisorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "advisor [directory]",
		Short: "Show optimization recommendations based on actual session data",
		Long: `Reads accumulated session data for a project and shows recommendations
for improving compression efficiency.`,
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

			stateDir := optimizer.DefaultStateDir(absDir)
			adv, err := optimizer.NewAdvisor(stateDir)
			if err != nil {
				return fmt.Errorf("load advisor: %w", err)
			}

			projectName := filepath.Base(absDir)
			fmt.Fprintf(os.Stdout, "\nEngram Advisor: %s\n", projectName)
			fmt.Fprintf(os.Stdout, "Sessions tracked: %d | Total turns: %d\n\n",
				adv.State.TotalSessions, adv.State.TotalTurns)

			recs := adv.Recommendations()
			for i, r := range recs {
				fmt.Fprintf(os.Stdout, "  %d. [%s] %s\n", i+1, r.Impact, r.Description)
			}
			fmt.Fprintln(os.Stdout)

			return nil
		},
	}
	return cmd
}
