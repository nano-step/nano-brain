package main

import (
	"strings"
	"testing"
)

func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"postgres URL", `  url: postgres://nb:hunter2@localhost:5432/db?sslmode=disable`, `  url: postgres://<redacted>@localhost:5432/db?sslmode=disable`},
		{"quoted postgres URL", `  url: "postgresql://nb:hunter2@localhost/db"`, `  url: "postgresql://<redacted>@localhost/db"`},
		{"keyword DSN", `  url: host=localhost user=nb password=hunter2 dbname=db`, `  url: host=localhost user=nb password=*** dbname=db`},
		{"quoted keyword DSN", `  url: "host=localhost password='hun ter2' dbname=db"`, `  url: "host=localhost password=*** dbname=db"`},
		{"api_key", `  api_key: sk-abc123`, `  api_key: ***`},
		{"voyage_api_key", `  voyage_api_key: "pa-abc123"`, `  voyage_api_key: ***`},
		{"password_hash", `      password_hash: "$2a$10$abcdefghijklmnopqrstuv"`, `      password_hash: ***`},
		{"password_hash as first key of a list item", `      - password_hash: $2a$10$abcdefghijklmnopqrstuv`, `      - password_hash: ***`},
		{"URL userinfo", `  url: https://user:hunter2@ollama.example/v1`, `  url: https://<redacted>@ollama.example/v1`},
		{"URL key parameter", `  provider_url: "https://llm.example/v1?api_key=abc123&model=x"`, `  provider_url: "https://llm.example/v1?api_key=<redacted>&model=x"`},
		{"URL token parameter", `  provider_url: https://llm.example/v1?Token=abc123 # note`, `  provider_url: https://llm.example/v1?Token=<redacted> # note`},
		{"plain URL", `  url: http://localhost:11434`, `  url: http://localhost:11434`},
		{"inline tokens", `    tokens: [nbt_one, "nbt_two"]`, `    tokens: ***`},
		{"max_tokens is not a token list", `  max_tokens: 500`, `  max_tokens: 500`},
		{"username kept", `      - username: admin`, `      - username: admin`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskSensitive(tt.in); got != tt.want {
				t.Errorf("maskSensitive(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMaskSensitive_Document(t *testing.T) {
	in := `server:
  host: 0.0.0.0
  port: 3199
  allowed_hosts:
    - brain.example
  auth:
    enabled: true
    realm: nano-brain
    users:
      - username: admin
        password_hash: "$2a$10$HASHHASHHASHHASHHASHHA"
    tokens:
      - nbt_FIRSTTOKEN
      # rotated monthly
      - "nbt_SECONDTOKEN"

      - nbt_THIRDTOKEN
    bypass_paths:
      - /health
database:
  url: postgres://nb:DBPASSWORD@localhost:5432/nanobrain_test
embedding:
  url: https://embed:EMBEDPASSWORD@ollama.example
  voyage_api_key: VOYAGEKEY
summarization:
  provider_url: https://llm.example/v1?key=QUERYKEY
  api_key: SUMMARYKEY
  max_tokens: 500
harvester:
  claudecode:
    ticket_patterns:
      - "[A-Z]+-\\d+"
`
	want := `server:
  host: 0.0.0.0
  port: 3199
  allowed_hosts:
    - brain.example
  auth:
    enabled: true
    realm: nano-brain
    users:
      - username: admin
        password_hash: ***
    tokens:
      - ***
      # rotated monthly
      - ***

      - ***
    bypass_paths:
      - /health
database:
  url: postgres://<redacted>@localhost:5432/nanobrain_test
embedding:
  url: https://<redacted>@ollama.example
  voyage_api_key: ***
summarization:
  provider_url: https://llm.example/v1?key=<redacted>
  api_key: ***
  max_tokens: 500
harvester:
  claudecode:
    ticket_patterns:
      - "[A-Z]+-\\d+"
`
	got := maskSensitive(in)
	if got != want {
		t.Errorf("maskSensitive() =\n%s\nwant\n%s", got, want)
	}
	for _, secret := range []string{"HASHHASH", "FIRSTTOKEN", "SECONDTOKEN", "THIRDTOKEN", "DBPASSWORD", "EMBEDPASSWORD", "VOYAGEKEY", "QUERYKEY", "SUMMARYKEY"} {
		if strings.Contains(got, secret) {
			t.Errorf("maskSensitive() output still contains %s", secret)
		}
	}
}
