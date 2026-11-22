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
// /private/var) still contains its own files. The target of an alias is
// admitted exactly as the walk would admit a regular file at that location:
// the filter plus the ignore files of every directory from root down to it
// (issue #675). filter may be nil, in which case the built-in credential deny
// list and those ignore files still apply.
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
		// The spelling says outside; ask the filesystem before refusing (#675).
		var ok bool
		if rel, ok = relBySameFile(realRoot, realPath); !ok {
			return ErrOutsideRoot
		}
	}
	// An in-root alias must not launder a file that is excluded under its own
	// name: docs/setup.md -> ../.env carries the content of .env.
	if isCredentialFile(filepath.Base(realPath)) {
		return ErrTargetDenied
	}
	// The filter works on paths under the unresolved root, so map the real
	// path back into it. Equal to path for anything that is not an alias,
	// which the caller's walk has already filtered.
	if target := filepath.Join(root, rel); target != path && walkWouldIgnore(root, filter, rel) {
		return ErrTargetDenied
	}
	return nil
}

// relBySameFile is the containment answer when filepath.Rel says realPath is
// outside realRoot. Rel compares spellings, so on a case-insensitive
// filesystem a root registered as /data/ws does not contain /Data/WS/a.md. It
// walks up from realPath looking for the directory that is realRoot
// (os.SameFile) and returns realPath relative to it. One Stat per path
// component, paid only by paths that look outside.
func relBySameFile(realRoot, realPath string) (string, bool) {
	rootInfo, err := os.Stat(realRoot)
	if err != nil {
		return "", false
	}
	rel := filepath.Base(realPath)
	for dir := filepath.Dir(realPath); ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && os.SameFile(rootInfo, info) {
			return rel, true
		}
		if dir == filepath.Dir(dir) {
			return "", false
		}
		rel = filepath.Join(filepath.Base(dir), rel)
	}
}

// walkWouldIgnore reports whether a walk of root would exclude the regular
// file at rel. It replays the walk's own gate (walkAdmitter) on every
// directory from root down to the file, so nested .gitignore /
// .nano-brainignore files count exactly as they do for a file that is not
// reached through an alias.
func walkWouldIgnore(root string, filter *FileFilter, rel string) bool {
	adm := newWalkAdmitter(filter)
	dir := root
	if adm.ignore(dir, true) {
		return true
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	for _, part := range parts[:len(parts)-1] {
		dir = filepath.Join(dir, part)
		if adm.ignore(dir, true) {
			return true
		}
	}
	return adm.ignore(filepath.Join(root, rel), false)
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
