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
		{"", true},
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

	err := checkBindSafety("0.0.0.0")
	if err == nil {
		t.Fatal("checkBindSafety(0.0.0.0) should return error without --unsafe-no-auth")
	}
}

func TestCheckBindSafety_AllowsLoopback(t *testing.T) {
	old := unsafeNoAuth
	unsafeNoAuth = false
	defer func() { unsafeNoAuth = old }()

	for _, host := range []string{"localhost", "127.0.0.1", "::1", ""} {
		if err := checkBindSafety(host); err != nil {
			t.Errorf("checkBindSafety(%q) returned unexpected error: %v", host, err)
		}
	}
}

func TestCheckBindSafety_UnsafeFlagBypasses(t *testing.T) {
	old := unsafeNoAuth
	unsafeNoAuth = true
	defer func() { unsafeNoAuth = old }()

	if err := checkBindSafety("0.0.0.0"); err != nil {
		t.Fatalf("checkBindSafety(0.0.0.0) with --unsafe-no-auth should not error: %v", err)
	}
}
