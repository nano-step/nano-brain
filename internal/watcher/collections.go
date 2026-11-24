package watcher

// collectionKey identifies one watched collection by (workspace, root path).
// Two workspaces may legitimately register collections on the same absolute
// path, so path alone cannot key watcher state (issue #714).
type collectionKey struct {
	workspaceHash string
	dirPath       string
}

// key returns the composite identity of col for the watcher state maps.
func (col watchedCollection) key() collectionKey {
	return collectionKey{workspaceHash: col.workspaceHash, dirPath: col.dirPath}
}

// IsLogicalCollection reports whether name is a DB-backed logical collection
// ("memory", "sessions") rather than a filesystem root. Logical collections
// must never be watched, walked, or fsnotify-scanned, and their names are
// reserved so users cannot shadow them with disk-backed collections (#714).
func IsLogicalCollection(name string) bool {
	return name == "memory" || name == "sessions"
}
