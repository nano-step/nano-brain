// Package clientauth protects bearer credentials on outbound HTTP requests.
package clientauth

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// SameOrigin compares scheme, hostname and effective port.
func SameOrigin(a, b string) bool {
	first, err := url.Parse(a)
	if err != nil || first.Hostname() == "" {
		return false
	}
	second, err := url.Parse(b)
	if err != nil || second.Hostname() == "" {
		return false
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		switch strings.ToLower(u.Scheme) {
		case "http":
			return "80"
		case "https":
			return "443"
		default:
			return ""
		}
	}
	return strings.EqualFold(first.Scheme, second.Scheme) &&
		strings.EqualFold(first.Hostname(), second.Hostname()) &&
		port(first) != "" && port(first) == port(second)
}

// CheckRemoteHTTP rejects bearer credentials sent over plaintext HTTP to a
// non-local host unless the operator explicitly opts in.
func CheckRemoteHTTP(u *url.URL) error {
	host := u.Hostname()
	ip := net.ParseIP(host)
	local := strings.EqualFold(host, "localhost") || strings.EqualFold(host, "host.docker.internal") || (ip != nil && ip.IsLoopback())
	if !strings.EqualFold(u.Scheme, "https") && !local && os.Getenv("NANO_BRAIN_ALLOW_INSECURE_REMOTE_TOKEN") != "1" {
		return errors.New("refusing to send credentials over remote HTTP; use a secure tunnel or set NANO_BRAIN_ALLOW_INSECURE_REMOTE_TOKEN=1")
	}
	return nil
}

// Do confines authenticated redirects to the original scheme and authority.
func Do(client *http.Client, req *http.Request) (*http.Response, error) {
	if req.Header.Get("Authorization") == "" {
		return client.Do(req)
	}
	if err := CheckRemoteHTTP(req.URL); err != nil {
		return nil, err
	}
	copy := *client
	copy.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		original := via[0].URL
		if !SameOrigin(next.URL.String(), original.String()) {
			return errors.New("refusing authenticated redirect to a different origin")
		}
		if client.CheckRedirect != nil {
			return client.CheckRedirect(next, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return copy.Do(req)
}
