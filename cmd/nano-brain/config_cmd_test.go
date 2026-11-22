package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"gopkg.in/yaml.v3"
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
		{"sslpassword keyword DSN", `  url: host=localhost sslpassword=hunter2 dbname=db`, `  url: host=localhost sslpassword=*** dbname=db`},
		{"keyword DSN password with ampersand", `  url: "host=x password=ab&cd dbname=db"`, `  url: "host=x password=*** dbname=db"`},
		{"keyword DSN password with quote", `  url: "host=x password=ab\"cd dbname=db"`, `  url: "host=x password=*** dbname=db"`},
		{"keyword DSN password keeps next params", `  url: "postgres://h/db?password=hunter2&sslmode=disable"`, `  url: "postgres://h/db?password=<redacted>&sslmode=disable"`},
		{"quoted keyword DSN", `  url: "host=localhost password='hun ter2' dbname=db"`, `  url: "host=localhost password=*** dbname=db"`},
		{"quoted DSN value with ampersand", `  url: "host=x password='a b&c' dbname=db"`, `  url: "host=x password=*** dbname=db"`},
		{"api_key", `  api_key: sk-abc123`, `  api_key: '***'`},
		{"voyage_api_key", `  voyage_api_key: "pa-abc123"`, `  voyage_api_key: '***'`},
		{"password_hash", `      password_hash: "$2a$10$abcdefghijklmnopqrstuv"`, `      password_hash: '***'`},
		{"password_hash as first key of a list item", `      - password_hash: $2a$10$abcdefghijklmnopqrstuv`, `      - password_hash: '***'`},
		{"URL userinfo", `  url: https://user:hunter2@ollama.example/v1`, `  url: https://<redacted>@ollama.example/v1`},
		{"URL key parameter", `  provider_url: "https://llm.example/v1?api_key=abc123&model=x"`, `  provider_url: "https://llm.example/v1?api_key=<redacted>&model=x"`},
		{"URL token parameter", `  provider_url: https://llm.example/v1?Token=abc123 # note`, `  provider_url: https://llm.example/v1?Token=<redacted> # note`},
		{"plain URL", `  url: http://localhost:11434`, `  url: http://localhost:11434`},
		{"inline tokens", `    tokens: [nbt_one, "nbt_two"]`, `    tokens: ['***', '***']`},
		{"max_tokens is not a token list", `  max_tokens: 500`, `  max_tokens: 500`},
		{"username kept", `      - username: admin`, `      - username: admin`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSensitive(tt.in)
			var actual, want interface{}
			if err := yaml.Unmarshal([]byte(got), &actual); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, want) {
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
        password_hash: '***'
    tokens:
      - '***'
      # rotated monthly
      - '***'

      - '***'
    bypass_paths:
      - /health
database:
  url: postgres://<redacted>@localhost:5432/nanobrain_test
embedding:
  url: https://<redacted>@ollama.example
  voyage_api_key: '***'
summarization:
  provider_url: https://llm.example/v1?key=<redacted>
  api_key: '***'
  max_tokens: 500
harvester:
  claudecode:
    ticket_patterns:
      - "[A-Z]+-\\d+"
`
	got := maskSensitive(in)
	var actual, expected interface{}
	if err := yaml.Unmarshal([]byte(got), &actual); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("maskSensitive() =\n%s\nwant\n%s", got, want)
	}
	for _, secret := range []string{"HASHHASH", "FIRSTTOKEN", "SECONDTOKEN", "THIRDTOKEN", "DBPASSWORD", "EMBEDPASSWORD", "VOYAGEKEY", "QUERYKEY", "SUMMARYKEY"} {
		if strings.Contains(got, secret) {
			t.Errorf("maskSensitive() output still contains %s", secret)
		}
	}
}

func TestMaskSensitive_YAMLLayouts(t *testing.T) {
	tests := []struct {
		name, input, secret string
	}{
		{"multiline flow tokens", "server:\n  auth:\n    tokens: [\n      FIRSTSECRET,\n      SECONDSECRET]\n", "FIRSTSECRET"},
		{"flow map", "auth: {enabled: true, tokens: [FLOWSECRET]}\n", "FLOWSECRET"},
		{"JSON style", `{"server":{"auth":{"tokens":["JSONSECRET"]}}}`, "JSONSECRET"},
		{"anchor list", "tokens: &t\n  - ANCHORSECRET\n", "ANCHORSECRET"},
		{"alias from elsewhere", "other: &t [ALIASSECRET]\ntokens: *t\n", "ALIASSECRET"},
		{"alias as key", "keys: &k password_hash\nusers:\n  - *k : ALIASKEYSECRET\n", "ALIASKEYSECRET"},
		{"case variant key in list item", "users:\n  - Password_Hash: CASEKEYSECRET\n", "CASEKEYSECRET"},
		{"commented out key", "# api_key: sk-COMMENTEDSECRET\napi_key: x\n", "COMMENTEDSECRET"},
		{"folded token", "tokens:\n  - >-\n    FOLDEDSECRET\n", "FOLDEDSECRET"},
		{"quoted key", "\"password_hash\": HASHSECRET\n", "HASHSECRET"},
		{"block scalar", "api_key: |-\n  BLOCKSECRET\n", "BLOCKSECRET"},
		{"value on next line", "api_key:\n  NEXTLINESECRET\n", "NEXTLINESECRET"},
		{"comment beside secret", "api_key: VALUE # COMMENTSECRET\n", "COMMENTSECRET"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSensitive(tt.input)
			if strings.Contains(got, tt.secret) {
				t.Errorf("credential still visible: %q", got)
			}
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(got), &doc); err != nil {
				t.Errorf("redacted output is not YAML: %v", err)
			}
		})
	}
}

func TestConfigShowRedactsURLsInJSONAndDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.URL = "postgres://user:DBSECRET@localhost:5432/db?sslmode=disable"
	cfg.Embedding.URL = "https://user:EMBEDSECRET@embed.example/v1?api_key=QUERYSECRET"
	data := configToMap(cfg, "config.yml")
	database := data["database"].(map[string]interface{})["url"].(string)
	embedding := data["embedding"].(map[string]interface{})["url"].(string)
	if database != "postgres://<redacted>@localhost:5432/db?sslmode=disable" || embedding != "https://<redacted>@embed.example/v1?api_key=<redacted>" {
		t.Errorf("JSON URL fields not fully redacted: %q, %q", database, embedding)
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	old := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = old }()
	printConfigDefaults(cfg)
	writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"DBSECRET", "EMBEDSECRET", "QUERYSECRET"} {
		if strings.Contains(string(output), secret) {
			t.Errorf("defaults output leaked %s", secret)
		}
	}
}

func TestMaskSensitive_BinaryTag(t *testing.T) {
	// koanf decodes !!binary before the value ever reaches a field, so the
	// base64 text itself is the credential and must be replaced wholesale.
	in := "database:\n  url: !!binary cG9zdGdyZXM6Ly91OlNFQ1JFVERERUBsb2NhbGhvc3Q6NTQzMi9kYg==\n"
	got := maskSensitive(in)
	if strings.Contains(got, "cG9zdGdyZXM6") || !strings.Contains(got, "<redacted>") {
		t.Errorf("maskSensitive() did not redact !!binary scalar: %q", got)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("redacted output is not YAML: %v", err)
	}
}

func TestMaskSensitive_InvalidYAML(t *testing.T) {
	if got := maskSensitive("api_key: sk-LEAKEDSECRET\n\t[unclosed\n"); got != "<redacted: invalid YAML>\n" {
		t.Fatalf("maskSensitive() = %q, want %q", got, "<redacted: invalid YAML>\n")
	}
}

func TestMaskSensitive_DSNTails(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		secrets []string
		kept    []string
	}{
		{"URL DSN second password param", "url: postgres://u@localhost:5432/d?password=FAKEPW1&sslpassword=SSLTAILSECRET\n", []string{"FAKEPW1", "SSLTAILSECRET"}, nil},
		{"keyword DSN ampersand key tail", "url: \"host=h password=abc&key=TAILSECRET dbname=d\"\n", []string{"TAILSECRET"}, []string{"dbname=d"}},
		{"keyword DSN trailing ampersand param", "url: \"password=abc&tail=KWTAIL\"\n", []string{"KWTAIL"}, nil},
		{"URL param keeps neighbours", "url: postgres://h/d?password=PWFIX&sslmode=disable\n", []string{"PWFIX"}, []string{"sslmode=disable"}},
		{"quoted value with ampersand", "url: \"host=h password='a b&c=d' dbname=d\"\n", []string{"b&c=d"}, []string{"dbname=d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSensitive(tt.input)
			for _, secret := range tt.secrets {
				if strings.Contains(got, secret) {
					t.Errorf("credential still visible: %q", got)
				}
			}
			for _, want := range tt.kept {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q to survive masking: %q", want, got)
				}
			}
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(got), &doc); err != nil {
				t.Errorf("redacted output is not YAML: %v", err)
			}
		})
	}
}

func TestMaskSensitive_BinaryKeys(t *testing.T) {
	tests := []struct {
		name, input, secret string
	}{
		{"binary voyage_api_key", "embedding:\n  !!binary dm95YWdlX2FwaV9rZXk= : BINSECRETA\n", "BINSECRETA"},
		{"binary tokens in flow map", "server: {auth: {enabled: true, !!binary dG9rZW5z : [nbt_BINKEYTOKEN_AAAAAAAAAAAAAAAAAAAAAAAA]}}\n", "nbt_BINKEYTOKEN_AAA"},
		{"alias to binary key", "defs:\n  k: &k !!binary dG9rZW5z\nserver:\n  auth:\n    enabled: true\n    *k : [nbt_ALIASBINTOKEN_BBBBBBBBBBBBBBBBBBBB]\n", "nbt_ALIASBINTOKEN_BBB"},
		{"binary password_hash key", "server:\n  auth:\n    enabled: true\n    users:\n      - username: admin\n        !!binary cGFzc3dvcmRfaGFzaA== : \"$2a$10$BINEKEYHASHSECRET\"\n", "BINEKEYHASHSECRET"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSensitive(tt.input)
			if strings.Contains(got, tt.secret) {
				t.Errorf("credential still visible: %q", got)
			}
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(got), &doc); err != nil {
				t.Errorf("redacted output is not YAML: %v", err)
			}
		})
	}
}

func TestConfigShow_BinaryKeysLoadAndStayHidden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := "server:\n  auth:\n    enabled: true\n    !!binary dG9rZW5z : [nbt_BINKEYREALTOKEN_0123456789ABCDEF]\nembedding:\n  !!binary dm95YWdlX2FwaV9rZXk= : BINVOYAGESECRET\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// koanf decodes the !!binary keys, so the server really loads these
	// credentials; that is what makes the masking necessary.
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() failed: %v", err)
	}
	if len(cfg.Server.Auth.Tokens) != 1 || cfg.Server.Auth.Tokens[0] != "nbt_BINKEYREALTOKEN_0123456789ABCDEF" {
		t.Fatalf("binary key did not load tokens: %#v", cfg.Server.Auth.Tokens)
	}
	if cfg.Embedding.VoyageAPIKey != "BINVOYAGESECRET" {
		t.Fatalf("binary key did not load voyage_api_key: %q", cfg.Embedding.VoyageAPIKey)
	}
	for _, jsonFlag := range []bool{false, true} {
		out := captureStdout(t, func() { runConfigShow(path, jsonFlag) })
		for _, secret := range []string{"nbt_BINKEYREALTOKEN", "BINVOYAGESECRET"} {
			if strings.Contains(out, secret) {
				t.Errorf("config show --json=%v leaked %s:\n%s", jsonFlag, secret, out)
			}
		}
	}
}

func TestConfigShow_DSNParserParity(t *testing.T) {
	cases := []struct {
		name    string
		content string
		secrets []string
	}{
		{"escaped space keyword value", "database:\n  url: \"host=h user=u password=ESC\\\\ TAILPW dbname=d\"\n", []string{"TAILPW"}},
		{"encoded sslpassword param", "database:\n  url: \"postgres://u@h/d?sslpass%77ord=ENCSSLPW&password=FAKEPW\"\n", []string{"ENCSSLPW", "FAKEPW"}},
		{"merge into database", "defs:\n  db: &db\n    url: \"host=h user='u'password=NOSPACEPW dbname=d\"\ndatabase:\n  <<: *db\n", []string{"NOSPACEPW"}},
		{"at sign in query user", "database:\n  url: \"postgres://h:5432/d?user=alice@example.com&password=ATLEAKQ1\"\n", []string{"ATLEAKQ1"}},
		{"at sign in query name", "database:\n  url: \"postgres://h/d?application_name=svc@host&password=ATLEAKQ2\"\n", []string{"ATLEAKQ2"}},
		{"uppercase scheme is keyword form", "database:\n  url: \"POSTGRES://h/d?x=1 password=UPLEAKQ3\"\n", []string{"UPLEAKQ3"}},
		{"leading space is keyword form", "database:\n  url: \" postgres://h/d?x=1 password=WSLEAKQ4\"\n", []string{"WSLEAKQ4"}},
		{"unterminated quote with backslash", "database:\n  url: \"host=h password='abc\\\\\"\n", []string{"abc"}},
		{"trailing backslash scalar", "embedding:\n  model: \"x='\\\\\"\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatalf("config.Load() failed: %v", err)
			}
			for _, s := range tc.secrets {
				if !strings.Contains(cfg.Database.URL, s) {
					t.Fatalf("config did not load the secret %q into database.url: %q", s, cfg.Database.URL)
				}
			}
			for _, jsonFlag := range []bool{false, true} {
				out := captureStdout(t, func() { runConfigShow(path, jsonFlag) })
				for _, s := range tc.secrets {
					if strings.Contains(out, s) {
						t.Errorf("config show --json=%v leaked %s:\n%s", jsonFlag, s, out)
					}
				}
			}
		})
	}
}
