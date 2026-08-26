package storage

import "testing"

func TestSourceFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"summary://claude/ses1", "claude"},
		{"summary://opencode/ses1", "opencode"},
		{"summary://pi/ses1", "pi"},
		{"opencode://ses1", "opencode"},
		{"claude://ses1", "claude"},
		{"summary://unknown-source/ses1", "unknown"},
	}
	for _, c := range cases {
		if got := SourceFromPath(c.path); got != c.want {
			t.Errorf("SourceFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
