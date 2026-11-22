package graph

import (
	"fmt"
	"io"
	"os"
)

// MaxAuxFileSize caps how many bytes ReadRegularFile will return. Manifests
// and tool configs (go.mod, package.json, tsconfig.json, ignore files) are far
// smaller than this; a larger file is treated as absent (#675).
const MaxAuxFileSize = 4 << 20

// ReadRegularFile reads path only when it resolves to a regular file of at
// most max bytes. Stat follows symlinks, so a symlink to a device, FIFO or
// socket is refused up front; the opened descriptor is re-checked so a swap
// between the two cannot slip a special file through (that window is the
// documented stat-then-open race). Reads are capped at max+1 bytes so an
// unbounded special file can never exhaust memory. Any failure — missing,
// non-regular, oversized, read error — returns an error, so callers treat the
// file exactly as they treat a missing one today.
func ReadRegularFile(path string, max int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("file exceeds %d bytes: %s", max, path)
	}
	return data, nil
}
