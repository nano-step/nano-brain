package symbol

import (
	"bytes"
	"strings"
)

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func lineForByte(src []byte, byteOffset uint32) int {
	if int(byteOffset) > len(src) {
		byteOffset = uint32(len(src))
	}
	return bytes.Count(src[:byteOffset], []byte{'\n'}) + 1
}
