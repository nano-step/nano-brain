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

// fillStrings sets every string field reachable from v to "SECRET:<dot.path>"
// and returns the dot-paths whose key name looks like a credential.
func fillStrings(v reflect.Value, prefix string, credentialPaths *[]string) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		name := field.Tag.Get("koanf")
		if name == "" {
			continue
		}
		path := strings.TrimPrefix(prefix+"."+name, ".")
		switch f := v.Field(i); f.Kind() {
		case reflect.Struct:
			fillStrings(f, path, credentialPaths)
		case reflect.String:
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
	var credentialPaths []string
	fillStrings(reflect.ValueOf(cfg).Elem(), "", &credentialPaths)
	if len(credentialPaths) < 7 {
		t.Fatalf("found only %d credential fields %v; the struct walk is broken", len(credentialPaths), credentialPaths)
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
	if strings.Contains(string(out), `"SECRET:database.url"`) {
		t.Error("database.url survives RedactSecrets")
	}
}
