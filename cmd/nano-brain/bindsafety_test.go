package main

import (
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

	for _, host := range []string{"", "   "} {
		if err := checkBindSafety(host, false); err == nil {
			t.Errorf("checkBindSafety(%q, authEnabled=false) returned nil; empty host is a wildcard bind and must require auth", host)
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
