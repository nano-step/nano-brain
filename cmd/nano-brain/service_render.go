package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// escapeXML escapes XML text content so user paths can never break the
// plist document.
func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

// renderLaunchdPlist renders the per-user LaunchAgent definition. The
// service always runs foreground `serve` (no -d) so launchd owns lifecycle
// and restart policy. Standard output/error go to explicit per-user paths
// under ~/.nano-brain/logs/.
func renderLaunchdPlist(label string, argv []string, logDir string) []byte {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	fmt.Fprintf(&b, "  <key>Label</key>\n  <string>%s</string>\n", escapeXML(label))
	b.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
	for _, a := range argv {
		fmt.Fprintf(&b, "    <string>%s</string>\n", escapeXML(a))
	}
	b.WriteString("  </array>\n")
	b.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	b.WriteString("  <key>KeepAlive</key>\n  <true/>\n")
	b.WriteString("  <key>ThrottleInterval</key>\n  <integer>5</integer>\n")
	fmt.Fprintf(&b, "  <key>StandardOutPath</key>\n  <string>%s</string>\n", escapeXML(filepath.Join(logDir, "service.log")))
	fmt.Fprintf(&b, "  <key>StandardErrorPath</key>\n  <string>%s</string>\n", escapeXML(filepath.Join(logDir, "service.err.log")))
	b.WriteString("</dict>\n</plist>\n")
	return []byte(b.String())
}

// escapeSystemdArg escapes one ExecStart argument per systemd.syntax:
// literal `%` becomes `%%` (specifier escape) and arguments containing
// whitespace or quotes are double-quoted with inner backslashes escaped.
func escapeSystemdArg(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")
	if strings.ContainsAny(s, " \t\"") {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return `"` + s + `"`
	}
	return s
}

// renderSystemdUnit renders the per-user systemd unit definition. The
// service always runs foreground `serve`; stdout/stderr go to the journal.
// No Environment= lines are emitted — the config file carries all durable
// settings, so no secret is ever serialized into the unit.
func renderSystemdUnit(unitName string, argv []string) []byte {
	escaped := make([]string, 0, len(argv))
	for _, a := range argv {
		escaped = append(escaped, escapeSystemdArg(a))
	}
	var b strings.Builder
	b.WriteString("[Unit]\n")
	b.WriteString("Description=nano-brain memory daemon\n")
	b.WriteString("After=network.target\n\n")
	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	fmt.Fprintf(&b, "ExecStart=%s\n", strings.Join(escaped, " "))
	b.WriteString("Restart=always\n")
	b.WriteString("RestartSec=2\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=default.target\n")
	return []byte(b.String())
}

// writeFileAtomic writes data to path via a same-directory temp file plus
// rename, so an interrupted write never leaves a half-written definition.
// Parent directories are created user-only (0700); the definition file
// itself is written with the requested permission.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename onto %s: %w", path, err)
	}
	return nil
}
