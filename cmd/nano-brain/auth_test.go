package main

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestAuthHash_ProducesValidBcrypt(t *testing.T) {
	password := "testpassword"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword failed: %v", err)
	}
	hashStr := string(hash)

	if !strings.HasPrefix(hashStr, "$2a$10$") {
		t.Errorf("expected bcrypt hash starting with $2a$10$, got %q", hashStr)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		t.Error("bcrypt hash does not verify against input password")
	}
}

func TestAuthToken_ProducesValidToken(t *testing.T) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	token := "nbt_" + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)

	if !strings.HasPrefix(token, "nbt_") {
		t.Errorf("expected token starting with nbt_, got %q", token)
	}

	encoded := strings.TrimPrefix(token, "nbt_")
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	if err != nil {
		t.Fatalf("token is not valid base64url: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("expected 32 random bytes, got %d", len(decoded))
	}
}
