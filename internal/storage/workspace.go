package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

func WorkspaceHash(rootPath string) string {
	absPath, _ := filepath.Abs(rootPath)
	h := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(h[:])
}
