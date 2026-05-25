package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

func WorkspaceHash(rootPath string) (string, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(h[:]), nil
}
