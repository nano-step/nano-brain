package config

import "testing"

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
