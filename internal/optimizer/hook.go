package optimizer

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pythondatascrape/engram/internal/session"
)

// DefaultStateDir returns ~/.engram/projects/<hash>/ for a given project directory.
func DefaultStateDir(projectDir string) string {
	home, _ := os.UserHomeDir()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(projectDir)))[:12]
	return filepath.Join(home, ".engram", "projects", hash)
}

// OnSessionComplete extracts stats from a completed session and records them in the advisor.
func OnSessionComplete(adv *Advisor, s *session.Session) error {
	snap := s.Snapshot()
	stats := SessionStats{
		Turns:               snap.Turns,
		IdentityTokensSaved: snap.TokensSaved,
		TotalTokensSent:     snap.TokensSent,
	}
	adv.RecordSession(stats)
	return adv.Save()
}
