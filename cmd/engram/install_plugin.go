package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pythondatascrape/engram/internal/install"
	"github.com/spf13/cobra"
)

const pluginVersion = "0.2.1"

func newInstallCmd() *cobra.Command {
	var (
		claudeOnly  bool
		openclawOnly bool
		sourceDir   string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install engram plugin into detected AI clients",
		Long: `Auto-detects installed AI clients (Claude Code, OpenClaw) and installs
the engram compression plugin. Use flags to target a specific client.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var targets []install.Client

			switch {
			case claudeOnly:
				client, found := install.DetectClaudeCode()
				if !found {
					return fmt.Errorf("Claude Code not detected (no ~/.claude/ directory)")
				}
				targets = append(targets, *client)

			case openclawOnly:
				client, found := install.DetectOpenClaw()
				if !found {
					return fmt.Errorf("OpenClaw not detected (no ~/.openclaw/ directory or openclaw in PATH)")
				}
				targets = append(targets, *client)

			default:
				targets = install.DetectAll()
				if len(targets) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No supported clients detected.")
					fmt.Fprintln(cmd.OutOrStdout())
					fmt.Fprintln(cmd.OutOrStdout(), "Supported clients:")
					fmt.Fprintln(cmd.OutOrStdout(), "  - Claude Code  (detected via ~/.claude/)")
					fmt.Fprintln(cmd.OutOrStdout(), "  - OpenClaw     (detected via ~/.openclaw/ or openclaw in PATH)")
					fmt.Fprintln(cmd.OutOrStdout())
					fmt.Fprintln(cmd.OutOrStdout(), "Install manually with:")
					fmt.Fprintln(cmd.OutOrStdout(), "  engram install --claude-code")
					fmt.Fprintln(cmd.OutOrStdout(), "  engram install --openclaw")
					return nil
				}
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
				fmt.Fprintf(cmd.OutOrStdout(), "Installing engram plugin for %s...\n", target.Name)

				pluginDir, err := install.PluginSourceDir(target.Name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  skipping %s: %v\n", target.Name, err)
					continue
				}

				src := filepath.Join(pluginBase, pluginDir)

				switch target.Name {
				case "claude-code":
					if err := install.RegisterClaudeCodeWithStatusline(src, pluginVersion, ""); err != nil {
						return fmt.Errorf("install Claude Code plugin: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  Claude Code plugin installed to ~/.claude/plugins/cache/engram/engram/%s/\n", pluginVersion)
					fmt.Fprintln(cmd.OutOrStdout(), "  Statusline registered in ~/.claude/settings.json")
					if runtime.GOOS == "darwin" {
						home, homeErr := os.UserHomeDir()
						binary, binErr := os.Executable()
						if homeErr != nil || binErr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "  warning: daemon service install skipped: %v %v\n", homeErr, binErr)
						} else {
							configPath := filepath.Join(home, ".engram", "engram.yaml")
							if err := installLaunchd(binary, configPath, ""); err != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "  warning: daemon service install failed: %v\n", err)
							} else {
								fmt.Fprintln(cmd.OutOrStdout(), "  Daemon installed as launchd service (starts on login)")
							}
						}
					}

				case "openclaw":
					if err := install.RegisterOpenClaw(src, pluginVersion); err != nil {
						return fmt.Errorf("install OpenClaw plugin: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  OpenClaw plugin installed to ~/.openclaw/plugins/engram/engram/%s/\n", pluginVersion)
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "\nDone. Restart your client to activate the engram plugin.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&claudeOnly, "claude-code", false, "Install Claude Code plugin only")
	cmd.Flags().BoolVar(&openclawOnly, "openclaw", false, "Install OpenClaw plugin only")
	cmd.Flags().StringVar(&sourceDir, "source", "", "Override plugin source directory")

	return cmd
}
