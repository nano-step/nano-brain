package watcher

import (
	"os"
	"path/filepath"
	"strings"
)

// PathInsideRoot reports whether path, once every symlink in it is resolved,
// is root itself or lies beneath it. root is resolved the same way, so a root
// reached through a symlink (macOS /var -> /private/var) still contains its own
// files. Containment is decided per path component: /data/ws does not contain
// /data/ws-evil/file. A path or root that cannot be resolved (broken symlink,
// missing file) is reported as not inside, with the error (issue #655).
func PathInsideRoot(root, path string) (bool, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil {
		return false, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return false, nil
	}
	return true, nil
}

// escapesRoot is the indexing-side gate around PathInsideRoot: it reports true,
// and logs why, when filePath must not be read because it does not resolve to a
// location inside root. Only paths are logged, never file contents.
func (w *Watcher) escapesRoot(root, filePath string) bool {
	inside, err := PathInsideRoot(root, filePath)
	if err != nil {
		w.logger.Debug().Err(err).Str("file", filePath).Msg("skipping file (path cannot be resolved)")
		return true
	}
	if !inside {
		w.logger.Warn().Str("file", filePath).Str("root", root).Msg("skipping symlink resolving outside workspace root")
		return true
	}
	return false
}
