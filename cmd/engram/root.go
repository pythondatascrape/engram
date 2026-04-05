package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "engram",
		Short:   "Engram — local-first context compression for AI coding tools",
		Long:    "Engram compresses LLM context through identity-aware serialization.\nOne binary saves 85-93% of redundant tokens across every LLM call.",
		Version: fmt.Sprintf("engram version %s", Version),
	}
	cmd.PersistentFlags().String("config", DefaultConfigPath(), "path to configuration file")
	cmd.PersistentFlags().String("socket", "", "Unix socket path (default: ~/.engram/engram.sock)")
	return cmd
}
