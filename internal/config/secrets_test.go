package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRedactSecrets_StripsAuthFields(t *testing.T) {
	cfg := getDefaults()
	cfg.Server.Auth.Users = []UserCred{
		{Username: "admin", PasswordHash: "$2a$10$secret"},
	}
	cfg.Server.Auth.Tokens = []string{"nbt_secret-token"}

	redacted := RedactSecrets(cfg)

	if len(redacted.Server.Auth.Users) != 0 {
		t.Errorf("expected Users to be nil/empty, got %d", len(redacted.Server.Auth.Users))
	}
	if len(redacted.Server.Auth.Tokens) != 0 {
		t.Errorf("expected Tokens to be nil/empty, got %d", len(redacted.Server.Auth.Tokens))
	}
	if redacted.Server.Auth.Enabled != cfg.Server.Auth.Enabled {
		t.Error("Enabled should be preserved in redacted config")
	}
	if redacted.Server.Auth.Realm != cfg.Server.Auth.Realm {
		t.Error("Realm should be preserved in redacted config")
	}
}

func TestRedactSecrets_PreservesOriginal(t *testing.T) {
	cfg := getDefaults()
	cfg.Server.Auth.Users = []UserCred{
		{Username: "admin", PasswordHash: "$2a$10$secret"},
	}
	cfg.Server.Auth.Tokens = []string{"nbt_token"}

	_ = RedactSecrets(cfg)

	if len(cfg.Server.Auth.Users) != 1 {
		t.Error("original Users should be unchanged")
	}
	if len(cfg.Server.Auth.Tokens) != 1 {
		t.Error("original Tokens should be unchanged")
	}
}

// urlCredential is the marker placed in the userinfo and in a key query
// parameter of every URL-named field by fillStrings.
func urlCredential(path string) string { return "URLCRED-" + path }

// fillStrings sets every string field reachable from v to "SECRET:<dot.path>"
// and returns the dot-paths whose key name looks like a credential. A field
// whose key name ends in "url" gets a URL carrying credentials instead, and
// its dot-path is appended to urlPaths.
func fillStrings(v reflect.Value, prefix string, credentialPaths, urlPaths *[]string) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		name := field.Tag.Get("koanf")
		if name == "" {
			continue
		}
		path := strings.TrimPrefix(prefix+"."+name, ".")
		switch f := v.Field(i); f.Kind() {
		case reflect.Struct:
			fillStrings(f, path, credentialPaths, urlPaths)
		case reflect.String:
			if strings.HasSuffix(name, "url") {
				f.SetString("https://user:" + urlCredential(path) + "@llm.example/v1?api_key=" + urlCredential(path))
				*urlPaths = append(*urlPaths, path)
				continue
			}
			f.SetString("SECRET:" + path)
			if strings.Contains(name, "api_key") || strings.Contains(name, "token") || strings.Contains(name, "password") || strings.Contains(name, "secret") {
				*credentialPaths = append(*credentialPaths, path)
			}
		}
	}
}

// Every credential-named string field in Config must be listed in
// SecretFieldPaths and blanked by RedactSecrets. Walking the struct means a
// new api_key field cannot be added without this test failing (#651).
func TestRedactSecrets_CoversEveryCredentialField(t *testing.T) {
	cfg := getDefaults()
	var credentialPaths, urlPaths []string
	fillStrings(reflect.ValueOf(cfg).Elem(), "", &credentialPaths, &urlPaths)
	if len(credentialPaths) < 7 {
		t.Fatalf("found only %d credential fields %v; the struct walk is broken", len(credentialPaths), credentialPaths)
	}
	if len(urlPaths) < 9 {
		t.Fatalf("found only %d URL fields %v; the struct walk is broken", len(urlPaths), urlPaths)
	}

	out, err := json.Marshal(RedactSecrets(cfg))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range credentialPaths {
		t.Run(path, func(t *testing.T) {
			if !IsSecretFieldPath(path) {
				t.Errorf("%s is not in SecretFieldPaths", path)
			}
			if strings.Contains(string(out), `"SECRET:`+path+`"`) {
				t.Errorf("%s survives RedactSecrets", path)
			}
		})
	}
	// A URL field must be blanked or passed through RedactURLCredentials, so
	// a new *url field cannot leak userinfo or a key parameter (#674).
	for _, path := range urlPaths {
		t.Run(path, func(t *testing.T) {
			if strings.Contains(string(out), urlCredential(path)) {
				t.Errorf("credentials in the %s URL survive RedactSecrets", path)
			}
		})
	}
}

func TestRedactURLCredentials(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "http://localhost:11434", "http://localhost:11434"},
		{"plain with path and query", "https://api.example/v1?model=x&stream=true", "https://api.example/v1?model=x&stream=true"},
		{"user and password", "https://user:pass@host/v1", "https://<redacted>@host/v1"},
		{"user only", "https://user@host:8443/v1", "https://<redacted>@host:8443/v1"},
		{"encoded at sign in password", "https://user:p%40ss@host/v1", "https://<redacted>@host/v1"},
		{"raw at sign in password", "https://user:p@ss@host/v1", "https://<redacted>@host/v1"},
		{"no scheme", "user:pass@host:11434/v1", "<redacted>@host:11434/v1"},
		{"at sign in path only", "https://host/users/a@b", "https://host/users/a@b"},
		{"api_key", "https://host/v1?api_key=abc123", "https://host/v1?api_key=<redacted>"},
		{"key among others", "https://host/v1?model=x&key=abc123&stream=true", "https://host/v1?model=x&key=<redacted>&stream=true"},
		{"token", "https://host/v1?token=abc123", "https://host/v1?token=<redacted>"},
		{"access_token", "https://host/v1?access_token=abc123", "https://host/v1?access_token=<redacted>"},
		{"secret", "https://host/v1?secret=abc123", "https://host/v1?secret=<redacted>"},
		{"password", "https://host/v1?password=abc123", "https://host/v1?password=<redacted>"},
		{"upper-case name", "https://host/v1?API_KEY=abc123", "https://host/v1?API_KEY=<redacted>"},
		{"percent-encoded name", "https://host/v1?api%5Fkey=abc123", "https://host/v1?api%5Fkey=<redacted>"},
		{"repeated parameter", "https://host/v1?key=abc&key=def", "https://host/v1?key=<redacted>&key=<redacted>"},
		{"name that only contains key", "https://host/v1?monkey=abc", "https://host/v1?monkey=abc"},
		{"fragment kept", "https://host/v1?key=abc#frag", "https://host/v1?key=<redacted>#frag"},
		{"scheme-like text in query", "host/v1?next=http://u:abc@x/", "host/v1?next=http://u:abc@x/"},
		{"userinfo and query", "https://u:pw@host/v1?token=abc", "https://<redacted>@host/v1?token=<redacted>"},
		{"unparsable with userinfo", "https://user:pa ss@host/v1", "<redacted>"},
		{"unparsable, slash in password", "https://user:p/ss@host/v1", "<redacted>"},
		{"unparsable, hash in password", "https://user:pa#ss@host/v1", "<redacted>"},
		{"unparsable without at sign", "http://host:port%zz/v1", "http://host:port%zz/v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactURLCredentials(tt.in); got != tt.want {
				t.Errorf("RedactURLCredentials(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// sharedReferences returns the dot-paths of every non-nil slice, map and
// pointer reachable from a that points at the same memory in b.
func sharedReferences(a, b reflect.Value, path string) []string {
	var shared []string
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			shared = append(shared, sharedReferences(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name)...)
		}
	case reflect.Slice:
		if a.Len() > 0 && b.Len() > 0 && a.Pointer() == b.Pointer() {
			shared = append(shared, path)
		}
		for i := 0; i < a.Len() && i < b.Len(); i++ {
			shared = append(shared, sharedReferences(a.Index(i), b.Index(i), path+"[]")...)
		}
	case reflect.Map:
		if !a.IsNil() && !b.IsNil() && a.Pointer() == b.Pointer() {
			return append(shared, path)
		}
		for _, key := range a.MapKeys() {
			if bv := b.MapIndex(key); bv.IsValid() {
				shared = append(shared, sharedReferences(a.MapIndex(key), bv, path+"[]")...)
			}
		}
	case reflect.Pointer:
		if !a.IsNil() && !b.IsNil() && a.Pointer() == b.Pointer() {
			shared = append(shared, path)
		}
	}
	return shared
}

// fillReferences gives every slice, map and pointer reachable from v one
// element, so a field RedactSecrets forgets to copy is caught by the walk.
func fillReferences(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fillReferences(v.Field(i))
		}
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillReferences(v.Index(0))
	case reflect.Map:
		elem := reflect.New(v.Type().Elem()).Elem()
		fillReferences(elem)
		v.Set(reflect.MakeMap(v.Type()))
		v.SetMapIndex(reflect.New(v.Type().Key()).Elem(), elem)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
	}
}

// The redacted copy must not share a slice, map or pointer with the live
// config: the handler serialises it while other goroutines read the original.
func TestRedactSecrets_SharesNoReferences(t *testing.T) {
	cfg := getDefaults()
	fillReferences(reflect.ValueOf(cfg).Elem())
	redacted := RedactSecrets(cfg)

	if shared := sharedReferences(reflect.ValueOf(cfg).Elem(), reflect.ValueOf(redacted).Elem(), "Config"); len(shared) > 0 {
		t.Errorf("RedactSecrets shares memory with the live config at: %v", shared)
	}
}

func TestRedactSecrets_MutatingTheCopyLeavesTheOriginal(t *testing.T) {
	writeToDisk := true
	cfg := getDefaults()
	cfg.Server.AllowedHosts = []string{"brain.example"}
	cfg.Server.Auth.BypassPaths = []string{"/health"}
	cfg.Harvester.ClaudeCode.TicketPatterns = []string{`[A-Z]+-\d+`}
	cfg.Watcher.ExcludePatterns = []string{"node_modules"}
	cfg.Watcher.AllowedExtensions = []string{".go"}
	cfg.Watcher.Workspaces = map[string]WorkspaceFilterConfig{
		"/data/workspaces/rails-app": {ExcludePatterns: []string{"tmp"}, AllowedExtensions: []string{".rb"}},
	}
	cfg.Search.HyDE.ContextHints = map[string]string{"rails-app": "a Rails monolith"}
	cfg.Summarization.WriteToDisk = &writeToDisk
	before, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	redacted := RedactSecrets(cfg)
	tests := []struct {
		name   string
		mutate func()
	}{
		{"server.allowed_hosts", func() { redacted.Server.AllowedHosts[0] = "evil.example" }},
		{"server.auth.bypass_paths", func() { redacted.Server.Auth.BypassPaths[0] = "/" }},
		{"harvester.claudecode.ticket_patterns", func() { redacted.Harvester.ClaudeCode.TicketPatterns[0] = ".*" }},
		{"watcher.exclude_patterns", func() { redacted.Watcher.ExcludePatterns[0] = "x" }},
		{"watcher.allowed_extensions", func() { redacted.Watcher.AllowedExtensions[0] = "x" }},
		{"watcher.workspaces entry", func() { redacted.Watcher.Workspaces["/data/workspaces/next-app"] = WorkspaceFilterConfig{} }},
		{"watcher.workspaces exclude_patterns", func() { redacted.Watcher.Workspaces["/data/workspaces/rails-app"].ExcludePatterns[0] = "x" }},
		{"watcher.workspaces allowed_extensions", func() { redacted.Watcher.Workspaces["/data/workspaces/rails-app"].AllowedExtensions[0] = "x" }},
		{"search.hyde.context_hints", func() { redacted.Search.HyDE.ContextHints["rails-app"] = "x" }},
		{"summarization.write_to_disk", func() { *redacted.Summarization.WriteToDisk = false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mutate()
			after, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Errorf("mutating the redacted copy changed the live config")
			}
		})
	}
}

// A nil slice or map must stay nil in the copy, so the JSON keeps "null".
func TestRedactSecrets_KeepsNilReferencesNil(t *testing.T) {
	cfg := getDefaults()
	cfg.Server.AllowedHosts = nil
	cfg.Watcher.Workspaces = nil
	cfg.Search.HyDE.ContextHints = nil
	cfg.Summarization.WriteToDisk = nil

	redacted := RedactSecrets(cfg)
	if redacted.Server.AllowedHosts != nil || redacted.Watcher.Workspaces != nil || redacted.Search.HyDE.ContextHints != nil || redacted.Summarization.WriteToDisk != nil {
		t.Errorf("nil reference fields became non-nil in the redacted copy")
	}
}
