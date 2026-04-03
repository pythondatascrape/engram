package main

import (
	"log/slog"
	"os"
)

func main() {
	rootCmd := newRootCmd()
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newAdvisorCmd())
	rootCmd.AddCommand(newInstallCmd())
	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

