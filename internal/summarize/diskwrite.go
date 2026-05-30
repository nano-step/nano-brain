package summarize

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureDir creates the parent directory of filePath (and all intermediate
// dirs) with mode 0o755. Returns nil if the directory already exists.
func EnsureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	return os.MkdirAll(dir, 0o755)
}

// writeFileAtomic writes content to path atomically: write to path+".tmp",
// fsync the tmp file, then os.Rename to final path. Prevents partial files
// on crash. Caller must ensure parent dir exists (call ensureDir first).
func WriteFileAtomic(path string, content []byte) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}

// resolveCollision returns the final write path, handling collisions:
//   - if path does not exist: returns path (no collision)
//   - if path exists AND content matches existing file byte-for-byte: returns path
//     (idempotent overwrite OK)
//   - if path exists with DIFFERENT content: returns path with "_<sha8>" suffix
//     where sha8 is first 8 chars of sha256(sessionID) hex, inserted before ".md"
// Caller writes to the returned path. SessionID is used to make the suffix
// stable across re-runs (same session always gets same suffix).
func ResolveCollision(path string, content []byte, sessionID string) (string, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil // no collision
		}
		return "", fmt.Errorf("stat existing: %w", err)
	}
	if string(existing) == string(content) {
		return path, nil // idempotent overwrite
	}
	// Collision: different content, append _<sha8>
	sum := sha256.Sum256([]byte(sessionID))
	suffix := "_" + hex.EncodeToString(sum[:4]) // 8 hex chars
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + suffix + ext, nil
}
