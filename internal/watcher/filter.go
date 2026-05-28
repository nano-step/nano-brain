package watcher

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

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
	excludePatterns   []string
	allowedExtensions map[string]bool
	rootDir           string
}

func newFileFilter(rootDir string, excludePatterns, allowedExtensions []string) *fileFilter {
	f := &fileFilter{
		rootDir:         rootDir,
		excludePatterns: excludePatterns,
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

	return f
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

	if f.gitignoreMatcher != nil && f.gitignoreMatcher.MatchesPath(rel) {
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
