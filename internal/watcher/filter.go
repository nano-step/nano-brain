package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/nano-brain/nano-brain/internal/graph"
)

// GitignoreStack maintains a stack of .gitignore matchers discovered during
// directory traversal. Each entry tracks the directory path and its associated
// gitignore matcher. The stack is used to apply nested .gitignore files in
// multi-repo workspaces (issue #379).
type GitignoreStack struct {
	entries []GitignoreEntry
}

type GitignoreEntry struct {
	dirPath string
	matcher *gitignore.GitIgnore
}

func (s *GitignoreStack) Push(dirPath string, matcher *gitignore.GitIgnore) {
	s.entries = append(s.entries, GitignoreEntry{
		dirPath: dirPath,
		matcher: matcher,
	})
}

// PopAbove removes stack entries that are not ancestors of the given path.
// This is called when ascending from a subdirectory during tree traversal.
func (s *GitignoreStack) PopAbove(currentPath string) {
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
func (s *GitignoreStack) Matches(path string) bool {
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

// walkAdmitter is the single file-admission gate shared by every WalkDir-based
// graph-build path (scanCollection and the Reextract* walks). It maintains the
// nested .gitignore/.nano-brainignore stack and applies the collection filter so
// the startup scan and the re-extract path can never diverge on which files are
// indexed (issue #535). Size is checked separately by callers that read the file.
// Not safe for concurrent use — construct one per WalkDir.
type walkAdmitter struct {
	filter *FileFilter
	stack  *GitignoreStack
}

func newWalkAdmitter(filter *FileFilter) *walkAdmitter {
	return &walkAdmitter{filter: filter, stack: &GitignoreStack{}}
}

// ignore updates the nested-ignore stack for the current walk position and
// reports whether the path is excluded. For a directory an ignored==true result
// means the caller must return filepath.SkipDir. Order mirrors scanCollection's
// original inline logic exactly: PopAbove → push nested ignores on dir entry →
// filter.ShouldSkip → stack.Matches.
func (a *walkAdmitter) ignore(path string, isDir bool) bool {
	a.stack.PopAbove(path)

	if isDir {
		if gi, err := compileRegularIgnore(filepath.Join(path, ".gitignore")); err == nil {
			a.stack.Push(path, gi)
		}
		if li, err := compileRegularIgnore(filepath.Join(path, ".nano-brainignore")); err == nil {
			a.stack.Push(path, li)
		}
	}

	if a.filter != nil && a.filter.ShouldSkip(path, isDir) {
		return true
	}
	return a.stack.Matches(path)
}

// collRelFile returns filePath as a slash-separated path relative to dirPath,
// matching the source_file form written by extractAndUpsertEdges. Falls back to
// filePath unchanged when it is not under dirPath.
func collRelFile(dirPath, filePath string) string {
	rel, err := filepath.Rel(dirPath, filePath)
	if err != nil {
		return filePath
	}
	return filepath.ToSlash(rel)
}

var defaultExcludeDirs = map[string]bool{
	"node_modules":  true,
	".git":          true,
	".hg":           true,
	".svn":          true,
	"dist":          true,
	"build":         true,
	"out":           true,
	".next":         true,
	".nuxt":         true,
	"vendor":        true,
	"__pycache__":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".tox":          true,
	"venv":          true,
	".venv":         true,
	"env":           true,
	".cache":        true,
	"coverage":      true,
	".terraform":    true,
	"target":        true,
	".worktrees":    true,
	".pr-reviews":   true,
	".opencode":     true,
	".output":       true,
	// Common framework build/vendor output dirs (Nuxt/Next/Svelte/Angular/
	// Astro/Turbo/Vercel/Netlify/Parcel and legacy JS package dirs).
	"_nuxt":            true,
	"_next":            true,
	".svelte-kit":      true,
	".angular":         true,
	".astro":           true,
	".turbo":           true,
	".vercel":          true,
	".netlify":         true,
	".parcel-cache":    true,
	"bower_components": true,
	"jspm_packages":    true,
	"web_modules":      true,
}

// defaultExcludeFiles are generated lock / resolved manifest files (by basename)
// that bloat the index with no search value. Files ending in ".lock" are also
// excluded (covers yarn.lock, Cargo.lock, Gemfile.lock, composer.lock,
// poetry.lock, Pipfile.lock, pubspec.lock, mix.lock, flake.lock, …).
var defaultExcludeFiles = map[string]bool{
	"package-lock.json":   true, // npm
	"npm-shrinkwrap.json": true, // npm
	"pnpm-lock.yaml":      true, // pnpm
	"bun.lockb":           true, // bun
	"packages.lock.json":  true, // .NET
	"go.sum":              true, // Go
	"Package.resolved":    true, // Swift
	"gradle.lockfile":     true, // Gradle
	".terraform.lock.hcl": true, // Terraform
	"Podfile.lock":        true, // CocoaPods (also .lock, kept explicit)
	"composer.lock":       true, // PHP (also .lock)
}

// Built-in credential deny list (issue #655). A file whose base name matches is
// never indexed, whatever exclude_patterns / allowed_extensions / ignore files
// say: the check runs before all of them and nothing after it can re-admit a
// file. Names are compared lower-cased. The same list is applied to the
// resolved target of a symlink (see checkResolvedPath).
//
// credentialFileNames are exact base names.
var credentialFileNames = map[string]bool{
	".env":             true,
	".envrc":           true,
	".npmrc":           true,
	".netrc":           true,
	".pgpass":          true,
	".htpasswd":        true,
	".git-credentials": true,
	".pypirc":          true,
	".dockercfg":       true,
	".vault-token":     true,
	".my.cnf":          true,
	".s3cfg":           true,
	".boto":            true,
	"kubeconfig":       true,
	"kubeconfig.yaml":  true,
	"kubeconfig.yml":   true,
	"htpasswd":         true,
	"credentials":      true, // as in ~/.aws/credentials
	"credentials.json": true,
	"credentials.yml":  true,
	"credentials.yaml": true,
	"secrets.yml":      true,
	"secrets.yaml":     true,
	"secrets.json":     true,
}

// credentialFileSuffixes deny any base name ending in one of them (*.pem, …).
var credentialFileSuffixes = []string{
	".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".ppk", ".kdbx", ".ovpn",
	".tfstate", ".tfstate.backup", ".tfvars", ".tfvars.json",
	".pem.bak", ".key.bak",
}

// sshKeyPrefixes deny any base name starting with one of them (id_rsa,
// id_rsa.bak, id_rsa_github, id_ed25519_sk) except "*.pub" public keys and
// names whose final extension is in sshKeySourceExts.
var sshKeyPrefixes = []string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519"}

// sshKeySourceExts are the final extensions that mark an id_rsa*-style name as
// source code or documentation (id_rsa_test.go, id_dsa.md) rather than a key.
// Data and text extensions (.txt, .json, .yml, …) are left out on purpose:
// id_rsa.txt is a plausible key backup.
var sshKeySourceExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".rb": true, ".rs": true, ".java": true, ".kt": true, ".c": true, ".h": true,
	".cpp": true, ".cs": true, ".php": true, ".swift": true, ".sh": true,
	".html": true, ".css": true, ".md": true,
}

// credentialJSONPrefixes deny "<prefix>*.json": OAuth client secrets and cloud
// service-account keys (client_secret_123.apps.googleusercontent.com.json).
var credentialJSONPrefixes = []string{"client_secret", "service-account", "service_account"}

// envDocWords mark an env file as documentation rather than secrets:
// ".env.<word>" / ".env.production.<word>" and "<word>.env" / "app.<word>.env".
var envDocWords = []string{"example", "sample", "template", "dist"}

// isCredentialFile reports whether base (a file's base name) is on the built-in
// credential deny list.
func isCredentialFile(base string) bool {
	name := strings.ToLower(base)
	if credentialFileNames[name] {
		return true
	}
	for _, suffix := range credentialFileSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	for _, prefix := range sshKeyPrefixes {
		if strings.HasPrefix(name, prefix) {
			return !strings.HasSuffix(name, ".pub") && !sshKeySourceExts[filepath.Ext(name)]
		}
	}
	if strings.HasSuffix(name, ".json") {
		for _, prefix := range credentialJSONPrefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}
	// .env-* / .env_* (.env-production, .env_local)
	if strings.HasPrefix(name, ".env-") || strings.HasPrefix(name, ".env_") {
		for _, word := range envDocWords {
			if strings.HasSuffix(name, "."+word) || strings.HasSuffix(name, "-"+word) || strings.HasSuffix(name, "_"+word) {
				return false
			}
		}
		return true
	}
	// .env.* (.env.local, .env.production)
	if strings.HasPrefix(name, ".env.") {
		for _, word := range envDocWords {
			if strings.HasSuffix(name, "."+word) {
				return false
			}
		}
		return true
	}
	// *.env (prod.env, app.env)
	if stem, ok := strings.CutSuffix(name, ".env"); ok {
		for _, word := range envDocWords {
			if stem == word || strings.HasSuffix(stem, "."+word) {
				return false
			}
		}
		return true
	}
	return false
}

type FileFilter struct {
	gitignoreMatcher  *gitignore.GitIgnore
	globalIgnore      *gitignore.GitIgnore
	localIgnore       *gitignore.GitIgnore
	excludePatterns   []string
	allowedExtensions map[string]bool
	rootDir           string
}

// NewFileFilter returns a filter for rootDir. The error reports IO failures
// while loading <rootDir>/.nano-brainignore (permission denied, is-a-directory,
// etc.). The returned *FileFilter is always valid, callers should log the
// error and continue with localIgnore unset.
func NewFileFilter(rootDir string, excludePatterns, allowedExtensions []string, globalIgnore *gitignore.GitIgnore) (*FileFilter, error) {
	f := &FileFilter{
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

	if gi, err := compileRegularIgnore(filepath.Join(rootDir, ".gitignore")); err == nil {
		f.gitignoreMatcher = gi
	}

	localIgnPath := filepath.Join(rootDir, ".nano-brainignore")
	gi, err := compileRegularIgnore(localIgnPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return f, fmt.Errorf("load workspace .nano-brainignore at %s: %w", localIgnPath, err)
		}
	} else {
		f.localIgnore = gi
	}

	return f, nil
}

func compileRegularIgnore(path string) (*gitignore.GitIgnore, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		if info.IsDir() {
			return nil, fmt.Errorf("ignore file is a directory: %s", path)
		}
		return nil, os.ErrNotExist
	}
	return gitignore.CompileIgnoreFile(path)
}

func (f *FileFilter) shouldSkip(absPath string, isDir bool) bool {
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

	// Exclude generated lock / resolved manifest files (by basename, plus any
	// *.lock). These bloat the index with no search value.
	if !isDir && len(parts) > 0 {
		base := parts[len(parts)-1]
		if defaultExcludeFiles[base] || strings.HasSuffix(base, ".lock") {
			return true
		}
		if isCredentialFile(base) {
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

func (f *FileFilter) ShouldSkip(absPath string, isDir bool) bool {
	return f.shouldSkip(absPath, isDir)
}

// LoadGlobalIgnore reads `<homeDir>/.nano-brain/.nano-brainignore` and returns
// a compiled gitignore matcher. Returns nil (without error) when the file is
// missing or malformed — the watcher must start regardless.
//
// homeDir is expected to be the absolute user home directory (callers should
// pass os.UserHomeDir() result). See issue #263.
func LoadGlobalIgnore(homeDir string) (*gitignore.GitIgnore, string, error) {
	path := filepath.Join(homeDir, ".nano-brain", ".nano-brainignore")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, nil
		}
		return nil, path, err
	}
	// A FIFO or device at this path is treated as absent rather than parsed:
	// opening it would block or read without bound (#675).
	if !info.Mode().IsRegular() {
		return nil, path, nil
	}
	data, err := graph.ReadRegularFile(path, graph.MaxAuxFileSize)
	if err != nil {
		return nil, path, err
	}
	return gitignore.CompileIgnoreLines(strings.Split(string(data), "\n")...), path, nil
}
