package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// gitignoreStack maintains a stack of .gitignore matchers discovered during
// directory traversal. Each entry tracks the directory path and its associated
// gitignore matcher. The stack is used to apply nested .gitignore files in
// multi-repo workspaces (issue #379).
type gitignoreStack struct {
	entries []gitignoreEntry
}

type gitignoreEntry struct {
	dirPath string
	matcher *gitignore.GitIgnore
}

func (s *gitignoreStack) Push(dirPath string, matcher *gitignore.GitIgnore) {
	s.entries = append(s.entries, gitignoreEntry{
		dirPath: dirPath,
		matcher: matcher,
	})
}

// PopAbove removes stack entries that are not ancestors of the given path.
// This is called when ascending from a subdirectory during tree traversal.
func (s *gitignoreStack) PopAbove(currentPath string) {
	i := 0
	for _, entry := range s.entries {
		rel, err := filepath.Rel(entry.dirPath, currentPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			s.entries[i] = entry
			i++
		}
	}
	s.entries = s.entries[:i]
}

// Matches checks if the given path matches any .gitignore pattern in the stack.
// Returns true if the path should be excluded.
func (s *gitignoreStack) Matches(path string) bool {
	for _, entry := range s.entries {
		rel, err := filepath.Rel(entry.dirPath, path)
		if err != nil {
			continue
		}
		if strings.HasPrefix(rel, "..") {
			continue
		}
		if entry.matcher.MatchesPath(rel) {
			return true
		}
	}
	return false
}

var defaultExcludeDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".hg":          true,
	".svn":         true,
	"dist":         true,
	"build":        true,
	"out":          true,
	".next":        true,
	".nuxt":        true,
	"vendor":       true,
	"__pycache__":  true,
	".pytest_cache": true,
	".mypy_cache":  true,
	".tox":         true,
	"venv":         true,
	".venv":        true,
	"env":          true,
	".cache":       true,
	"coverage":     true,
	".terraform":   true,
	"target":       true,
}

type fileFilter struct {
	gitignoreMatcher  *gitignore.GitIgnore
	globalIgnore      *gitignore.GitIgnore
	localIgnore       *gitignore.GitIgnore
	excludePatterns   []string
	allowedExtensions map[string]bool
	rootDir           string
}

// newFileFilter returns a filter for rootDir. The error reports IO failures
// while loading <rootDir>/.nano-brainignore (permission denied, is-a-directory,
// etc.). The returned *fileFilter is always valid; callers should log the
// error and continue with localIgnore unset.
func newFileFilter(rootDir string, excludePatterns, allowedExtensions []string, globalIgnore *gitignore.GitIgnore) (*fileFilter, error) {
	f := &fileFilter{
		rootDir:         rootDir,
		excludePatterns: excludePatterns,
		globalIgnore:    globalIgnore,
	}

	if len(allowedExtensions) > 0 {
		f.allowedExtensions = make(map[string]bool, len(allowedExtensions))
		for _, ext := range allowedExtensions {
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			f.allowedExtensions[strings.ToLower(ext)] = true
		}
	}

	gitignorePath := filepath.Join(rootDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		if gi, err := gitignore.CompileIgnoreFile(gitignorePath); err == nil {
			f.gitignoreMatcher = gi
		}
	}

	localIgnPath := filepath.Join(rootDir, ".nano-brainignore")
	gi, err := gitignore.CompileIgnoreFile(localIgnPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return f, fmt.Errorf("load workspace .nano-brainignore at %s: %w", localIgnPath, err)
		}
	} else {
		f.localIgnore = gi
	}

	return f, nil
}

func (f *fileFilter) shouldSkip(absPath string, isDir bool) bool {
	rel, err := filepath.Rel(f.rootDir, absPath)
	if err != nil {
		rel = absPath
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		if defaultExcludeDirs[part] {
			return true
		}
	}

	matchRel := rel
	if isDir && rel != "." && !strings.HasSuffix(matchRel, "/") {
		matchRel = matchRel + "/"
	}

	if f.globalIgnore != nil && f.globalIgnore.MatchesPath(matchRel) {
		return true
	}

	if f.localIgnore != nil && f.localIgnore.MatchesPath(matchRel) {
		return true
	}

	if f.gitignoreMatcher != nil && f.gitignoreMatcher.MatchesPath(matchRel) {
		return true
	}

	for _, pattern := range f.excludePatterns {
		matched, err := filepath.Match(pattern, filepath.Base(absPath))
		if err == nil && matched {
			return true
		}
		if strings.Contains(pattern, "/") {
			matched, err = filepath.Match(pattern, rel)
			if err == nil && matched {
				return true
			}
		}
	}

	if len(f.allowedExtensions) > 0 && !isDir {
		ext := strings.ToLower(filepath.Ext(absPath))
		if !f.allowedExtensions[ext] {
			return true
		}
	}

	return false
}

// LoadGlobalIgnore reads `<homeDir>/.nano-brain/.nano-brainignore` and returns
// a compiled gitignore matcher. Returns nil (without error) when the file is
// missing or malformed — the watcher must start regardless.
//
// homeDir is expected to be the absolute user home directory (callers should
// pass os.UserHomeDir() result). See issue #263.
func LoadGlobalIgnore(homeDir string) (*gitignore.GitIgnore, string, error) {
	path := filepath.Join(homeDir, ".nano-brain", ".nano-brainignore")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, path, nil
		}
		return nil, path, err
	}
	gi, err := gitignore.CompileIgnoreFile(path)
	if err != nil {
		return nil, path, err
	}
	return gi, path, nil
}
