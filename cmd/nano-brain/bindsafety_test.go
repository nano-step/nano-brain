package main

import (
	"slices"
	"testing"
)

func TestIsLoopback(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.5", true},
		{"::1", true},
		{"[::1]", true},
		// An empty host is NOT loopback: Server.Start renders it as ":<port>",
		// which binds every interface. This case previously asserted true,
		// which is why the hole survived — the test encoded the bug as the
		// specification. See #635.
		{"", false},
		{"   ", false},
		// A bare bracket pair is a third spelling of the same wildcard, and
		// the one config.Load does NOT normalize (TrimSpace("[]") != ""), so
		// this layer is its only defense. strings.Trim strips it to "", which
		// pre-fix made isLoopback report true — while
		// net.SplitHostPort("[]:3199") yields host "" with no error, i.e. a
		// bind on every interface. Unauthenticated wildcard bind, different
		// spelling.
		{"[]", false},
		{"[::]", false},
		{"::", false},
		{"LOCALHOST", true},
		{"0.0.0.0", false},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"example.com", false},
	}
	for _, tc := range cases {
		got := isLoopback(tc.host)
		if got != tc.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestCheckBindSafety_RejectsNonLoopback(t *testing.T) {
	old := unsafeNoAuth
	unsafeNoAuth = false
	defer func() { unsafeNoAuth = old }()

	err := checkBindSafety("0.0.0.0", false)
	if err == nil {
		t.Fatal("checkBindSafety(0.0.0.0) should return error without --unsafe-no-auth")
	}
}

func TestCheckBindSafety_AllowsLoopback(t *testing.T) {
	old := unsafeNoAuth
	unsafeNoAuth = false
	defer func() { unsafeNoAuth = old }()

	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		if err := checkBindSafety(host, false); err != nil {
			t.Errorf("checkBindSafety(%q) returned unexpected error: %v", host, err)
		}
	}
}

// TestCheckBindSafety_RejectsEmptyHost pins #635: an empty host renders as
// ":<port>" in Server.Start and binds every interface, so it must be gated
// exactly like "0.0.0.0". checkBindSafety previously substituted "localhost"
// for it and returned nil — a security control failing open on a value that a
// bare `host:` in YAML produces.
func TestCheckBindSafety_RejectsEmptyHost(t *testing.T) {
	old := unsafeNoAuth
	unsafeNoAuth = false
	defer func() { unsafeNoAuth = old }()

	// "[]" is included because it is the spelling config.Load does not
	// normalize — this check is the only thing standing between it and an
	// unauthenticated bind on every interface.
	for _, host := range []string{"", "   ", "[]", "[::]"} {
		if err := checkBindSafety(host, false); err == nil {
			t.Errorf("checkBindSafety(%q, authEnabled=false) returned nil; this is a wildcard bind and must require auth", host)
		}
		// The documented escapes must still work, so an operator who really
		// wants a wildcard bind is not stuck.
		if err := checkBindSafety(host, true); err != nil {
			t.Errorf("checkBindSafety(%q, authEnabled=true) = %v, want nil", host, err)
		}
	}
}

func TestCheckBindSafety_UnsafeFlagBypasses(t *testing.T) {
	old := unsafeNoAuth
	unsafeNoAuth = true
	defer func() { unsafeNoAuth = old }()

	if err := checkBindSafety("0.0.0.0", false); err != nil {
		t.Fatalf("checkBindSafety(0.0.0.0) with --unsafe-no-auth should not error: %v", err)
	}
}

func TestCheckBindSafety_AuthEnabledBypasses(t *testing.T) {
	old := unsafeNoAuth
	unsafeNoAuth = false
	defer func() { unsafeNoAuth = old }()

	if err := checkBindSafety("0.0.0.0", true); err != nil {
		t.Fatalf("checkBindSafety(0.0.0.0) with auth enabled should not error: %v", err)
	}
}

// A bypass path is unauthenticated by design. On a network-reachable bind
// with auth on, anything beyond the two defaults is flagged so the operator
// sees it at startup (#673).
func TestUnauthenticatedBypassPaths(t *testing.T) {
	custom := []string{"/health", "/api/openapi.json", "/api/v1/write", "/mcp"}

	tests := []struct {
		name        string
		host        string
		authEnabled bool
		bypass      []string
		want        []string
	}{
		{"non-loopback with auth flags extra paths", "0.0.0.0", true, custom, []string{"/api/v1/write", "/mcp"}},
		{"wildcard spelled empty", "", true, custom, []string{"/api/v1/write", "/mcp"}},
		{"defaults only", "0.0.0.0", true, []string{"/health", "/api/openapi.json"}, nil},
		{"no bypass paths", "0.0.0.0", true, nil, nil},
		{"loopback bind", "127.0.0.1", true, custom, nil},
		{"auth disabled", "0.0.0.0", false, custom, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unauthenticatedBypassPaths(tt.host, tt.authEnabled, tt.bypass)
			if !slices.Equal(got, tt.want) {
				t.Errorf("unauthenticatedBypassPaths(%q, %v, %v) = %v, want %v", tt.host, tt.authEnabled, tt.bypass, got, tt.want)
			}
		})
	}
}
