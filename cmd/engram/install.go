package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/spf13/cobra"
)

var (
	launchdTmpl = template.Must(template.New("plist").Parse(launchdPlistTemplate))
	systemdTmpl = template.Must(template.New("unit").Parse(systemdUnitTemplate))
)

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.engram.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Binary}}</string>
		<string>serve</string>
		<string>--foreground</string>
		<string>--config</string>
		<string>{{.ConfigPath}}</string>
		<string>--socket</string>
		<string>{{.SocketPath}}</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogDir}}/engram.log</string>
	<key>StandardErrorPath</key>
	<string>{{.LogDir}}/engram.err</string>
</dict>
</plist>
`

const systemdUnitTemplate = `[Unit]
Description=Engram context compression daemon
After=network.target

[Service]
Type=simple
ExecStart={{.Binary}} serve --config {{.ConfigPath}} --socket {{.SocketPath}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

type serviceTemplateData struct {
	Binary     string
	ConfigPath string
	SocketPath string
	LogDir     string
}

func renderTemplate(tmpl *template.Template, data serviceTemplateData) string {
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, data)
	return buf.String()
}

func generateLaunchdPlist(binary, configPath, socketPath string) string {
	return renderTemplate(launchdTmpl, serviceTemplateData{
		Binary:     binary,
		ConfigPath: configPath,
		SocketPath: socketPath,
		LogDir:     filepath.Join(filepath.Dir(configPath), "logs"),
	})
}

func generateSystemdUnit(binary, configPath, socketPath string) string {
	return renderTemplate(systemdTmpl, serviceTemplateData{
		Binary:     binary,
		ConfigPath: configPath,
		SocketPath: socketPath,
	})
}

func writeServiceFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func installService(cmd *cobra.Command) error {
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	configPath, _ := cmd.Flags().GetString("config")
	socketPath, _ := cmd.Flags().GetString("socket")

	if f := cmd.Flags().Lookup("config"); !f.Changed {
		configPath = DefaultConfigPath()
	} else if !filepath.IsAbs(configPath) {
		wd, _ := os.Getwd()
		configPath = filepath.Join(wd, configPath)
	}
	if f := cmd.Flags().Lookup("socket"); !f.Changed {
		socketPath = DefaultSocketPath()
	}

	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(binary, configPath, socketPath)
	case "linux":
		return installSystemd(binary, configPath, socketPath)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func installLaunchd(binary, configPath, socketPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.engram.daemon.plist")
	content := generateLaunchdPlist(binary, configPath, socketPath)

	if err := writeServiceFile(plistPath, content); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", plistPath)

	// Load the agent.
	out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %s: %w", string(out), err)
	}
	fmt.Fprintln(os.Stderr, "engram daemon installed and loaded via launchd")
	return nil
}

func installSystemd(binary, configPath, socketPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	unitPath := filepath.Join(home, ".config", "systemd", "user", "engram.service")
	content := generateSystemdUnit(binary, configPath, socketPath)

	if err := writeServiceFile(unitPath, content); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", unitPath)

	// Enable and start.
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", string(out), err)
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", "engram").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable: %s: %w", string(out), err)
	}
	fmt.Fprintln(os.Stderr, "engram daemon installed and enabled via systemd")
	return nil
}
