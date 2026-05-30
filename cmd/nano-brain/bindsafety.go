package main

import (
	"fmt"
	"net"
	"strings"
)

// unsafeNoAuth is set by the --unsafe-no-auth flag on the serve subcommand.
var unsafeNoAuth bool

func isLoopback(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]"))
	if h == "" || h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func checkBindSafety(host string) error {
	if host == "" {
		host = "localhost"
	}
	if isLoopback(host) {
		return nil
	}
	if unsafeNoAuth {
		return nil
	}
	return fmt.Errorf(
		"server.host=%q binds to a non-loopback address without authentication. "+
			"This exposes your memory to anyone on the network. Either bind to "+
			"localhost/127.0.0.1/::1 OR pass --unsafe-no-auth to acknowledge the risk",
		host,
	)
}
