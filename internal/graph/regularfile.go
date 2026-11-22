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
// that substitutes a device or socket is still refused. It does not close
// the stat-then-open race: a swap that substitutes a FIFO blocks in open(2)
// before the re-check runs, and that window is the documented residual gap.
// Reads are capped at max+1 bytes so a regular file that reports size 0 —
// /proc/self/pagemap on Linux reads for hundreds of GiB — cannot exhaust
// memory. Any failure — missing, non-regular, oversized, read error —
// returns an error, so callers treat the file exactly as they treat a
// missing one today.
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
	if fi.Size() > max {
		return nil, fmt.Errorf("file exceeds %d bytes: %s", max, path)
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
