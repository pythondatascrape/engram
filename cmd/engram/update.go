package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pythondatascrape/engram/internal/install"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var sourceDir string

	return &cobra.Command{
		Use:   "update",
		Short: "Update engram plugin in all detected AI clients",
		Long: `Re-installs the engram plugin for all detected AI clients.
Run this after upgrading the engram binary to push new plugin files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			targets := install.DetectAll()
			if len(targets) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No supported clients detected.")
				return nil
			}

			pluginBase := sourceDir
			if pluginBase == "" {
				exe, err := os.Executable()
				if err != nil {
					return fmt.Errorf("cannot determine executable path: %w", err)
				}
				pluginBase = filepath.Dir(exe)
			}

			for _, target := range targets {
				fmt.Fprintf(cmd.OutOrStdout(), "Updating engram plugin for %s...\n", target.Name)

				pluginDir, err := install.PluginSourceDir(target.Name)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  skipping %s: %v\n", target.Name, err)
					continue
				}

				src := filepath.Join(pluginBase, pluginDir)

				switch target.Name {
				case "claude-code":
					if err := install.RegisterClaudeCodeWithStatusline(src, pluginVersion, ""); err != nil {
						return fmt.Errorf("update Claude Code plugin: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  Updated to v%s\n", pluginVersion)
					if runtime.GOOS == "darwin" {
						home, homeErr := os.UserHomeDir()
						binary, binErr := os.Executable()
						if homeErr != nil || binErr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "  warning: daemon service update skipped: %v %v\n", homeErr, binErr)
						} else {
							configPath := filepath.Join(home, ".engram", "engram.yaml")
							if err := installLaunchd(binary, configPath, ""); err != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "  warning: daemon service update failed: %v\n", err)
							}
						}
					}

				case "openclaw":
					if err := install.RegisterOpenClaw(src, pluginVersion); err != nil {
						return fmt.Errorf("update OpenClaw plugin: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  Updated to v%s\n", pluginVersion)
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "\nDone. Restart your client to activate the update.")
			return nil
		},
	}
}
