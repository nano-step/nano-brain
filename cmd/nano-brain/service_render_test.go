package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscapeXML(t *testing.T) {
	got := escapeXML(`a<b>&"c"'d`)
	want := "a&lt;b&gt;&amp;&quot;c&quot;&apos;d"
	if got != want {
		t.Errorf("escapeXML = %q, want %q", got, want)
	}
}

func TestRenderLaunchdPlist(t *testing.T) {
	label := "com.nano-step.nano-brain"
	argv := []string{"/opt/homebrew/bin/node", "/opt/homebrew/lib/node_modules/@nano-step/nano-brain/npm/run.js", "--config", "/home/user/.nano-brain/config.yml", "serve"}
	logDir := "/home/user/.nano-brain/logs"

	data := renderLaunchdPlist(label, argv, logDir)

	// The document must be well-formed XML, and the label must appear as the
	// first <string> value after <key>Label</key>.
	var doc struct {
		Dict struct {
			Keys []string `xml:"key"`
		} `xml:"dict"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("plist is not valid XML: %v\n%s", err, string(data))
	}
	labelSeen := false
	for _, k := range doc.Dict.Keys {
		if k == "Label" {
			labelSeen = true
		}
	}
	if !labelSeen {
		t.Errorf("plist missing Label key:\n%s", string(data))
	}

	s := string(data)
	for _, want := range []string{
		"<key>RunAtLoad</key>", "<key>KeepAlive</key>", "<key>ThrottleInterval</key>",
		"<integer>5</integer>",
		"com.nano-step.nano-brain", "/home/user/.nano-brain/config.yml", "serve",
		"/home/user/.nano-brain/logs/service.log", "/home/user/.nano-brain/logs/service.err.log",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q", want)
		}
	}
	if strings.Contains(s, "--daemon-child") || strings.Contains(s, " -d ") {
		t.Error("plist must launch foreground serve, never the detached daemon")
	}
}

func TestRenderLaunchdPlistEscapesPaths(t *testing.T) {
	data := renderLaunchdPlist("com.nano-step.nano-brain", []string{"/bin/echo", "/home/user/a&b/c<d>"}, "/home/user/.nano-brain/logs")
	s := string(data)
	if strings.Contains(s, "a&b") || strings.Contains(s, "c<d>") {
		t.Errorf("plist must XML-escape user paths:\n%s", s)
	}
	if !strings.Contains(s, "a&amp;b") || !strings.Contains(s, "c&lt;d&gt;") {
		t.Errorf("plist missing escaped path:\n%s", s)
	}
}

func TestEscapeSystemdArg(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/usr/bin/nano-brain", "/usr/bin/nano-brain"},
		{"/home/user/my config.yml", `"/home/user/my config.yml"`},
		{`/path/with"quote`, `"/path/with\"quote"`},
		{"80%", "80%%"},
		{"/path with space%", `"/path with space%%"`},
	}
	for _, tt := range tests {
		if got := escapeSystemdArg(tt.in); got != tt.want {
			t.Errorf("escapeSystemdArg(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderSystemdUnit(t *testing.T) {
	argv := []string{"/usr/bin/node", "/home/user/.config/nano-brain/npm/run.js", "--config", "/home/user/.nano-brain/config.yml", "serve"}
	data := string(renderSystemdUnit("nano-brain", argv))

	for _, want := range []string{
		"[Unit]", "[Service]", "[Install]",
		"Type=simple",
		"ExecStart=/usr/bin/node /home/user/.config/nano-brain/npm/run.js --config /home/user/.nano-brain/config.yml serve",
		"Restart=always", "RestartSec=2",
		"WantedBy=default.target",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("unit missing %q:\n%s", want, data)
		}
	}
	if strings.Contains(data, "Environment=") {
		t.Error("unit must not serialize environment variables")
	}
	if strings.Contains(data, "-d") {
		t.Error("unit must launch foreground serve, never the detached daemon")
	}
}

func TestRenderSystemdUnitEscapesArgs(t *testing.T) {
	argv := []string{"/bin/echo", "/home/user/my config.yml"}
	data := string(renderSystemdUnit("nano-brain", argv))
	want := `ExecStart=/bin/echo "/home/user/my config.yml"`
	if !strings.Contains(data, want) {
		t.Errorf("unit should quote args with spaces, got:\n%s", data)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "deep", "com.nano-step.nano-brain.plist")
	if err := writeFileAtomic(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want hello", string(got))
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o777 != 0o644 {
		t.Errorf("file mode = %o, want 644", info.Mode()&0o777)
	}
	parent := filepath.Dir(filepath.Dir(target))
	pi, _ := os.Stat(parent)
	if pi.Mode()&0o777 != 0o700 {
		t.Errorf("parent dir mode = %o, want 700 (user-only)", pi.Mode()&0o777)
	}
	// No temp files may remain after a successful write.
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func TestWriteFileAtomicOverwritesExisting(t *testing.T) {
	target := filepath.Join(t.TempDir(), "def")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(target, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("content = %q, want new", string(got))
	}
}
