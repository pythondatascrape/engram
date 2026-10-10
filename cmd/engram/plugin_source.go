package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePluginDir returns the directory holding a client's plugin files, where
// pluginDir is relative to the engram root (e.g. "plugins/claude-code"). An
// explicit sourceDir (the --source flag) wins; otherwise the files are looked up
// relative to the running binary.
func resolvePluginDir(sourceDir, pluginDir string) (string, error) {
	if sourceDir != "" {
		return filepath.Join(sourceDir, pluginDir), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	p, err := findNearBinary(exe, pluginDir)
	if err != nil {
		return "", fmt.Errorf("%w (pass --source <engram checkout>)", err)
	}
	return p, nil
}

// findNearBinary follows symlinks on exe (e.g. ~/.local/bin/engram -> checkout/bin/engram)
// and returns the first existing <base>/rel, trying the binary's directory, its
// parent (a checkout's bin/), and its grandparent.
func findNearBinary(exe, rel string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	candidates := []string{dir, filepath.Dir(dir), filepath.Dir(filepath.Dir(dir))}
	for _, base := range candidates {
		p := filepath.Join(base, rel)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found under %s", rel, strings.Join(candidates, ", "))
}
