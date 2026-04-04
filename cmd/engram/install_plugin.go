package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pythondatascrape/engram/internal/install"
	"github.com/spf13/cobra"
)

const pluginVersion = "0.2.0"

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
					fmt.Println("No supported clients detected.")
					fmt.Println()
					fmt.Println("Supported clients:")
					fmt.Println("  - Claude Code  (detected via ~/.claude/)")
					fmt.Println("  - OpenClaw     (detected via ~/.openclaw/ or openclaw in PATH)")
					fmt.Println()
					fmt.Println("Install manually with:")
					fmt.Println("  engram install --claude-code")
					fmt.Println("  engram install --openclaw")
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
				fmt.Printf("Installing engram plugin for %s...\n", target.Name)

				pluginDir, err := install.PluginSourceDir(target.Name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  skipping %s: %v\n", target.Name, err)
					continue
				}

				src := filepath.Join(pluginBase, pluginDir)

				switch target.Name {
				case "claude-code":
					if err := install.RegisterClaudeCode(src, pluginVersion); err != nil {
						return fmt.Errorf("install Claude Code plugin: %w", err)
					}
					fmt.Printf("  Claude Code plugin installed to ~/.claude/plugins/cache/engram/engram/%s/\n", pluginVersion)

				case "openclaw":
					if err := install.RegisterOpenClaw(src, pluginVersion); err != nil {
						return fmt.Errorf("install OpenClaw plugin: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  OpenClaw plugin installed to ~/.openclaw/plugins/engram/engram/%s/\n", pluginVersion)
				}
			}

			fmt.Println("\nDone. Restart your client to activate the engram plugin.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&claudeOnly, "claude-code", false, "Install Claude Code plugin only")
	cmd.Flags().BoolVar(&openclawOnly, "openclaw", false, "Install OpenClaw plugin only")
	cmd.Flags().StringVar(&sourceDir, "source", "", "Override plugin source directory")

	return cmd
}
