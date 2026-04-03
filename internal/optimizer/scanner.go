package optimizer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectType identifies the detected project language/framework.
type ProjectType string

const (
	ProjectTypeGo      ProjectType = "go"
	ProjectTypeNode    ProjectType = "node"
	ProjectTypePython  ProjectType = "python"
	ProjectTypeRust    ProjectType = "rust"
	ProjectTypeUnknown ProjectType = "unknown"
)

// IdentityFile represents a discovered identity/context file.
type IdentityFile struct {
	Name       string // relative path from project root
	Path       string // absolute path
	Content    string // raw file content
	TokenCount int    // estimated token count (chars / 4)
	SizeBytes  int    // raw byte size
}

// ProjectProfile is the result of scanning a project directory.
type ProjectProfile struct {
	Dir           string
	Type          ProjectType
	IdentityFiles []IdentityFile
	TotalTokens   int // sum of all identity file tokens
}

// identityFileNames lists files to scan for identity content.
var identityFileNames = []string{
	"CLAUDE.md",
	"AGENTS.md",
	".claude/CLAUDE.md",
	".cursorrules",
	".github/copilot-instructions.md",
}

// ScanProject scans a directory and returns a ProjectProfile.
func ScanProject(dir string) (*ProjectProfile, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan project: %q is not a directory", dir)
	}

	profile := &ProjectProfile{Dir: dir}
	profile.Type = detectProjectType(dir)

	for _, name := range identityFileNames {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		tokens := estimateTokens(string(data))
		idf := IdentityFile{
			Name:       name,
			Path:       path,
			Content:    string(data),
			TokenCount: tokens,
			SizeBytes:  len(data),
		}
		profile.IdentityFiles = append(profile.IdentityFiles, idf)
		profile.TotalTokens += tokens
	}

	return profile, nil
}

func detectProjectType(dir string) ProjectType {
	markers := []struct {
		file string
		typ  ProjectType
	}{
		{"go.mod", ProjectTypeGo},
		{"package.json", ProjectTypeNode},
		{"pyproject.toml", ProjectTypePython},
		{"requirements.txt", ProjectTypePython},
		{"Cargo.toml", ProjectTypeRust},
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			return m.typ
		}
	}
	return ProjectTypeUnknown
}

// estimateTokens approximates token count using chars/4 heuristic.
func estimateTokens(s string) int {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}
	tokens := len(s) / 4
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}
