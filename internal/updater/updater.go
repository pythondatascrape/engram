// Package updater checks for newer Engram releases on GitHub and applies
// them in-place. When an update is found the binary replaces itself and
// the process exits so launchd/systemd can restart it with the new version.
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	releaseAPI = "https://api.github.com/repos/pythondatascrape/engram/releases/latest"
	httpTimeout = 15 * time.Second
)

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckAndApply fetches the latest GitHub release. If it is newer than
// current, the binary is replaced in-place and the process exits so the
// supervisor (launchd/systemd) can restart it. Safe to call in a goroutine.
func CheckAndApply(current string) {
	rel, err := fetchLatest()
	if err != nil {
		slog.Debug("updater: fetch failed", "error", err)
		return
	}

	if !isNewer(current, rel.TagName) {
		slog.Debug("updater: already up to date", "version", current)
		return
	}

	slog.Info("updater: new version available", "current", current, "latest", rel.TagName)

	url := assetURL(rel)
	if url == "" {
		slog.Warn("updater: no matching asset for platform", "os", runtime.GOOS, "arch", runtime.GOARCH)
		return
	}

	if err := applyUpdate(url); err != nil {
		slog.Error("updater: apply failed", "error", err)
		return
	}

	slog.Info("updater: update applied, restarting", "version", rel.TagName)
	os.Exit(0) // launchd/systemd will restart with the new binary
}

func fetchLatest() (*release, error) {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(releaseAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel release
	return &rel, json.NewDecoder(resp.Body).Decode(&rel)
}

// isNewer returns true if latest != current (simple string comparison;
// both are semver tags like "v0.2.0"). Treats "dev" as always stale.
func isNewer(current, latest string) bool {
	if current == "dev" || current == "" {
		return false // don't auto-update dev builds
	}
	return latest != current && latest != ""
}

// assetURL finds the download URL for this OS/arch combination.
func assetURL(rel *release) string {
	want := fmt.Sprintf("engram_%s_%s", runtime.GOOS, runtime.GOARCH)
	for _, a := range rel.Assets {
		if strings.Contains(strings.ToLower(a.Name), strings.ToLower(want)) {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

// applyUpdate downloads the asset to a temp file, then atomically replaces
// the running binary.
func applyUpdate(url string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("eval symlinks: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(filepath.Dir(exe), ".engram-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	return os.Rename(tmp.Name(), exe)
}
