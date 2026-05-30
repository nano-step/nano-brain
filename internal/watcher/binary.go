package watcher

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// binaryExtensions lists file extensions whose content is known to be binary.
// Files matched by this map are skipped before any disk read or DB write
// (issue #252). The list is intentionally hardcoded — operators who need
// additional extensions can rely on the utf8.Valid safety net in
// isBinaryContent, which catches anything not in this list.
var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".bmp": true, ".ico": true, ".tiff": true, ".heic": true, ".heif": true,
	".pdf": true, ".psd": true, ".ai": true, ".sketch": true,
	".zip": true, ".tar": true, ".gz": true, ".7z": true, ".rar": true,
	".bz2": true, ".xz": true,
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true,
	".mp3": true, ".wav": true, ".flac": true, ".ogg": true, ".aac": true, ".m4a": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true,
	".bin": true, ".obj": true, ".wasm": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".db": true, ".sqlite": true, ".sqlite3": true,
}

// isBinaryExtension reports whether filePath has an extension known to contain
// binary content. The check is case-insensitive.
func isBinaryExtension(filePath string) bool {
	return binaryExtensions[strings.ToLower(filepath.Ext(filePath))]
}

// isBinaryContent reports whether the byte slice contains a sequence that is
// not valid UTF-8. PostgreSQL TEXT columns reject such sequences with
// SQLSTATE 22021; this check prevents reaching that error.
func isBinaryContent(content []byte) bool {
	return !utf8.Valid(content)
}
