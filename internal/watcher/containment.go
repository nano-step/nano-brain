package watcher

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Reasons CheckResolvedPath refuses a path. A resolve failure is returned as
// the underlying error instead (fs.ErrNotExist for a broken symlink).
var (
	ErrOutsideRoot  = errors.New("path resolves outside workspace root")
	ErrTargetDenied = errors.New("path resolves to an excluded file")
)

// CheckResolvedPath decides whether path may be read for indexing once every
// symlink in it is resolved (issue #655). It returns nil when the real path is
// inside root's real path and is itself a file the filter would admit;
// otherwise ErrOutsideRoot, ErrTargetDenied, or the resolve error. root is
// resolved too, so a root reached through a symlink (macOS /var ->
// /private/var) still contains its own files. filter may be nil, in which case
// only the built-in credential deny list is applied to the target.
func CheckResolvedPath(root string, filter *FileFilter, path string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	return checkResolvedPath(root, realRoot, filter, path)
}

// checkResolvedPath is CheckResolvedPath with root already resolved, so a walk
// resolves the root once instead of once per file.
func checkResolvedPath(root, realRoot string, filter *FileFilter, path string) error {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	// Containment is decided per path component: /data/ws does not contain
	// /data/ws-evil/file.
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return ErrOutsideRoot
	}
	// An in-root alias must not launder a file that is excluded under its own
	// name: docs/setup.md -> ../.env carries the content of .env.
	if isCredentialFile(filepath.Base(realPath)) {
		return ErrTargetDenied
	}
	if filter != nil {
		// The filter works on paths under the unresolved root, so map the real
		// path back into it. Equal to path for anything that is not an alias,
		// which the caller's walk has already filtered.
		if target := filepath.Join(root, rel); target != path && filter.ShouldSkip(target, false) {
			return ErrTargetDenied
		}
	}
	return nil
}

// skipResolved is the watcher-side gate around checkResolvedPath. It reports
// whether filePath must not be read and whether a document indexed for it
// earlier should be purged: yes when the path escapes the root, resolves to an
// excluded file, or no longer resolves at all; no for any other resolve error
// (EIO, ELOOP, …), which may be transient. Only paths are logged.
func (w *Watcher) skipResolved(col watchedCollection, filePath string) (skip, purge bool) {
	realRoot := col.realRoot
	if realRoot == "" {
		var err error
		if realRoot, err = filepath.EvalSymlinks(col.dirPath); err != nil {
			w.logger.Warn().Err(err).Str("file", filePath).Str("root", col.dirPath).Msg("skipping file (workspace root cannot be resolved)")
			return true, false
		}
	}
	err := checkResolvedPath(col.dirPath, realRoot, col.filter, filePath)
	switch {
	case err == nil:
		return false, false
	case errors.Is(err, ErrOutsideRoot):
		// A symlink to a directory (bazel-*, Nix result, …) was never indexed:
		// no Warn and nothing to purge.
		if target, statErr := os.Stat(filePath); statErr == nil && target.IsDir() {
			w.logger.Debug().Str("file", filePath).Msg("skipping symlink to directory")
			return true, false
		}
		// Warn once per path per process; every later scan repeats at Debug.
		ev := w.logger.Debug()
		if w.firstSymlinkWarning(filePath) {
			ev = w.logger.Warn()
		}
		ev.Str("file", filePath).Str("root", col.dirPath).Msg("skipping symlink resolving outside workspace root")
		return true, true
	case errors.Is(err, ErrTargetDenied):
		w.logger.Debug().Str("file", filePath).Msg("skipping symlink resolving to an excluded file")
		return true, true
	case errors.Is(err, fs.ErrNotExist):
		w.logger.Debug().Err(err).Str("file", filePath).Msg("skipping file (path cannot be resolved)")
		return true, true
	default:
		w.logger.Warn().Err(err).Str("file", filePath).Msg("skipping file (path cannot be resolved)")
		return true, false
	}
}

// firstSymlinkWarning reports whether filePath has not been warned about yet in
// this process, and records it.
func (w *Watcher) firstSymlinkWarning(filePath string) bool {
	w.symlinkWarnedMu.Lock()
	defer w.symlinkWarnedMu.Unlock()
	if w.symlinkWarned[filePath] {
		return false
	}
	if w.symlinkWarned == nil {
		w.symlinkWarned = make(map[string]bool)
	}
	w.symlinkWarned[filePath] = true
	return true
}
