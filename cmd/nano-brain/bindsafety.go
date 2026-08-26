package main

import (
	"fmt"
	"net"
	"strings"
)

// unsafeNoAuth is set by the --unsafe-no-auth flag on the serve subcommand.
var unsafeNoAuth bool

// serveOnlyFlag is set by the --serve-only flag on the serve subcommand and
// overrides cfg.Server.ServeOnly at startup. See issue #282.
var serveOnlyFlag bool

// isLoopback reports whether host names a loopback address.
//
// An empty host is NOT loopback. Server.Start builds its listen address with
// fmt.Sprintf("%s:%d", host, port), so an empty host produces ":<port>", which
// binds every interface — the same exposure as "0.0.0.0", which this function
// correctly rejects. Treating "" as localhost made the safety check model the
// bind as safer than it actually was, and a security control that fails open
// on a plausible typo is worse than no control.
func isLoopback(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]"))
	if h == "" {
		return false
	}
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func checkBindSafety(host string, authEnabled bool) error {
	if isLoopback(host) {
		return nil
	}
	if authEnabled {
		return nil
	}
	if unsafeNoAuth {
		return nil
	}
	return fmt.Errorf(
		"server.host=%q binds to a non-loopback address without authentication. "+
			"This exposes your memory to anyone on the network. Either bind to "+
			"localhost/127.0.0.1/::1, configure auth, OR pass --unsafe-no-auth to acknowledge the risk",
		host,
	)
}
