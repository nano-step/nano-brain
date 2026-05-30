package watcher

import "testing"

func TestIsBinaryExtension(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"image.png", true},
		{"photo.jpg", true},
		{"photo.jpeg", true},
		{"animation.gif", true},
		{"document.pdf", true},
		{"archive.zip", true},
		{"video.mp4", true},
		{"audio.mp3", true},
		{"library.so", true},
		{"font.woff2", true},
		{"data.sqlite", true},
		{"data.sqlite3", true},

		{"Image.PNG", true},
		{"photo.JPG", true},
		{"Archive.Zip", true},

		{"notes.md", false},
		{"main.go", false},
		{"app.ts", false},
		{"config.yml", false},
		{"schema.sql", false},
		{"data.json", false},
		{"readme.txt", false},
		{"setup.toml", false},
		{"script.sh", false},
		{"index.html", false},
		{"styles.css", false},

		{"unknown.xyz", false},
		{"no-extension", false},
		{"", false},
	}

	for _, c := range cases {
		got := isBinaryExtension(c.path)
		if got != c.want {
			t.Errorf("isBinaryExtension(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsBinaryContent(t *testing.T) {
	cases := []struct {
		name    string
		content []byte
		want    bool
	}{
		{"empty", []byte{}, false},
		{"valid ascii", []byte("# Title\n\nSome markdown text."), false},
		{"valid utf8 vietnamese", []byte("Chào thế giới — Tóm tắt"), false},
		{"valid utf8 emoji", []byte("Hello 👋 world"), false},

		{"png magic bytes", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, true},
		{"jpeg soi marker", []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}, true},
		{"pdf header", []byte("%PDF-1.4\n%\xff\xff\xff\xff"), true},
		{"gif header", []byte("GIF89a\xff\xff\x00\x00"), true},

		{"mixed valid then invalid", append([]byte("Hello "), 0x89, 0x50, 0x4e, 0x47), true},

		{"null byte alone", []byte{0x00}, true},
		{"null byte in middle of utf8 text", []byte("Hello\x00World"), true},
		{"null byte at end", []byte("trailing\x00"), true},
	}

	for _, c := range cases {
		got := isBinaryContent(c.content)
		if got != c.want {
			t.Errorf("isBinaryContent(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
