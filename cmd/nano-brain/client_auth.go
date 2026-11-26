package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/nano-brain/nano-brain/internal/clientauth"
	"github.com/nano-brain/nano-brain/internal/config"
)

func init() {
	httpClient.Transport = bearerTransport{base: http.DefaultTransport}
	httpClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		authenticated := clientToken() != "" || via[0].Header.Get("Authorization") != ""
		if authenticated && !clientauth.SameOrigin(next.URL.String(), via[0].URL.String()) {
			return errors.New("refusing authenticated redirect to a different origin")
		}
		return nil
	}
}

type bearerTransport struct {
	base http.RoundTripper
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := clientToken()
	if token != "" && req.Header.Get("Authorization") == "" {
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Header.Set("Authorization", "Bearer "+token)
		req = clone
	}
	if req.Header.Get("Authorization") != "" {
		if err := clientauth.CheckRemoteHTTP(req.URL); err != nil {
			return nil, err
		}
	}
	if t.base == nil {
		t.base = http.DefaultTransport
	}
	return t.base.RoundTrip(req)
}

func clientToken() string {
	if token := os.Getenv("NANO_BRAIN_TOKEN"); token != "" {
		return token
	}
	cfg, err := config.Load(resolveClientConfigPath())
	if err == nil && len(cfg.Server.Auth.Tokens) > 0 {
		return cfg.Server.Auth.Tokens[0]
	}
	return ""
}

func setClientAuthorization(req *http.Request) {
	if token := clientToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
