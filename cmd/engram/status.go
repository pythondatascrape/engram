package main

import (
	"fmt"
	"os"

	"github.com/pythondatascrape/engram/internal/daemon"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Engram daemon status",
		Long:  "Connect to the running Engram daemon and display health information\nincluding active sessions, total turns processed, tokens saved, and uptime.",
		RunE:  runStatus,
	}
	cmd.Flags().String("socket", defaultSocketPath(), "Unix socket path")
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	socketPath, _ := cmd.Flags().GetString("socket")

	client, err := daemon.NewClient(socketPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Engram daemon is not running.\nStart it with: engram serve\nSocket: %s\n", socketPath)
		os.Exit(1)
	}
	defer client.Close()

	health, err := client.Health()
	if err != nil {
		return fmt.Errorf("failed to get daemon status: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Engram Daemon Status\n")
	fmt.Fprintf(out, "====================\n")
	fmt.Fprintf(out, "Status:          %s\n", health.Status)
	fmt.Fprintf(out, "Uptime:          %s\n", health.Uptime)
	fmt.Fprintf(out, "Active Sessions: %d\n", health.ActiveSessions)
	fmt.Fprintf(out, "Total Turns:     %d\n", health.TotalTurns)
	fmt.Fprintf(out, "Tokens Saved:    %d\n", health.TotalSaved)
	fmt.Fprintln(out)
	return nil
}
