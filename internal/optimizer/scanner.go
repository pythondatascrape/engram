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

// skipDirs are directories that should not be walked for identity files.
var skipDirs = map[string]bool{
	".git": true, ".claude": true, ".github": true, ".venv": true,
	"node_modules": true, "__pycache__": true, "vendor": true,
	"dist": true, ".worktrees": true, ".next": true,
}

// ScanProject scans a directory tree and returns a ProjectProfile.
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

	// Check root-only files (dotfile paths that don't repeat in subdirs).
	for _, name := range rootOnlyFiles {
		path := filepath.Join(dir, name)
		if idf, ok := readIdentityFile(dir, path, name); ok {
			profile.IdentityFiles = append(profile.IdentityFiles, idf)
			profile.TotalTokens += idf.TokenCount
		}
	}

	// Walk the tree for files that can appear at any level.
	filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			if skipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		for _, name := range recursiveFileNames {
			if fi.Name() == name {
				rel, _ := filepath.Rel(dir, path)
				if idf, ok := readIdentityFile(dir, path, rel); ok {
					profile.IdentityFiles = append(profile.IdentityFiles, idf)
					profile.TotalTokens += idf.TokenCount
				}
			}
		}
		return nil
	})

	return profile, nil
}

// recursiveFileNames are identity files that can appear at any directory level.
var recursiveFileNames = []string{"CLAUDE.md", "AGENTS.md", ".cursorrules"}

// rootOnlyFiles are identity files checked only at the project root.
var rootOnlyFiles = []string{".claude/CLAUDE.md", ".github/copilot-instructions.md"}

func readIdentityFile(dir, path, name string) (IdentityFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return IdentityFile{}, false
	}
	tokens := estimateTokens(string(data))
	return IdentityFile{
		Name:       name,
		Path:       path,
		TokenCount: tokens,
		SizeBytes:  len(data),
	}, true
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
