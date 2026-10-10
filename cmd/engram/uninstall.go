package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/pythondatascrape/engram/internal/config"
	"github.com/pythondatascrape/engram/internal/install"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	var purge bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove everything `engram install` set up",
		Long: `Reverses ` + "`engram install`" + `: stops and removes the background service,
removes the Claude Code and OpenClaw plugins, and removes the entries engram
added to ~/.claude/settings.json (statusline, Stop hook, proxy env vars and
request header). Unrelated settings are left untouched.

~/.engram (config, logs, session stats) is kept unless --purge is given.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if err := uninstallService(runtime.GOOS, out); err != nil {
				return err
			}

			proxyPorts := []int{config.DefaultProxyPort, config.DefaultOpenAIProxyPort}
			if cfg, err := config.Load(DefaultConfigPath()); err == nil {
				proxyPorts = append(proxyPorts, cfg.Proxy.Port, cfg.Proxy.OpenAIPort)
			}
			changed, err := install.UnregisterClaudeSettings("", proxyPorts...)
			if err != nil {
				return fmt.Errorf("clean Claude Code settings: %w", err)
			}
			if changed {
				fmt.Fprintln(out, "Removed engram entries from ~/.claude/settings.json")
			}

			if err := install.UnregisterClaudeCode(); err != nil {
				return fmt.Errorf("remove Claude Code plugin: %w", err)
			}
			fmt.Fprintln(out, "Removed Claude Code plugin")

			if err := install.UnregisterOpenClaw(); err != nil {
				return fmt.Errorf("remove OpenClaw plugin: %w", err)
			}

			_ = os.Remove(DefaultSocketPath())

			if purge {
				dir := filepath.Dir(DefaultConfigPath())
				if err := os.RemoveAll(dir); err != nil {
					return fmt.Errorf("remove %s: %w", dir, err)
				}
				fmt.Fprintf(out, "Removed %s\n", dir)
			}

			fmt.Fprintln(out, "\nDone. Restart your client so it stops routing through engram.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false, "Also delete ~/.engram (config, logs, session stats)")
	return cmd
}

// uninstallService stops and removes the launchd agent or systemd user unit
// written by installLaunchd/installSystemd. Missing services are not an error.
func uninstallService(goos string, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	switch goos {
	case "darwin":
		plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.engram.daemon.plist")
		if _, err := os.Stat(plistPath); os.IsNotExist(err) {
			return nil
		}
		// Unload fails when the agent is not loaded; removing the plist is what matters.
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		if err := os.Remove(plistPath); err != nil {
			return fmt.Errorf("remove plist: %w", err)
		}
		fmt.Fprintf(out, "Stopped daemon and removed %s\n", plistPath)

	case "linux":
		unitPath := filepath.Join(home, ".config", "systemd", "user", "engram.service")
		if _, err := os.Stat(unitPath); os.IsNotExist(err) {
			return nil
		}
		_ = exec.Command("systemctl", "--user", "disable", "--now", "engram").Run()
		if err := os.Remove(unitPath); err != nil {
			return fmt.Errorf("remove unit: %w", err)
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		fmt.Fprintf(out, "Stopped daemon and removed %s\n", unitPath)
	}
	return nil
}
