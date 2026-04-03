package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := newRootCmd()
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newVersionCmd())
	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{Use: "serve", Short: "Start the Engram daemon"}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show Engram daemon status"}
}
