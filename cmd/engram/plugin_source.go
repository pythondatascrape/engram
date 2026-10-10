package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePluginBase returns the directory that contains plugins/<client>/.
// An explicit sourceDir (the --source flag) wins; otherwise the location is
// derived from the running binary.
func resolvePluginBase(sourceDir, pluginDir string) (string, error) {
	if sourceDir != "" {
		return sourceDir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	return findPluginBase(exe, pluginDir)
}

// findPluginBase follows symlinks on exe (e.g. ~/.local/bin/engram -> checkout/bin/engram)
// and checks, in order, the binary's directory, its parent (a checkout's bin/), and its
// grandparent for pluginDir.
func findPluginBase(exe, pluginDir string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	candidates := []string{dir, filepath.Dir(dir), filepath.Dir(filepath.Dir(dir))}
	for _, base := range candidates {
		if info, err := os.Stat(filepath.Join(base, pluginDir)); err == nil && info.IsDir() {
			return base, nil
		}
	}
	return "", fmt.Errorf("plugin files %s not found under %s (pass --source <engram checkout>)",
		pluginDir, strings.Join(candidates, ", "))
}
