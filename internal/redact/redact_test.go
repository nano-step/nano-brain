package redact

import (
	"strings"
	"testing"
)

func TestScrub_LeavesNormalTextUnchanged(t *testing.T) {
	cases := []string{
		"",
		"func handleRequest(ctx context.Context) error { return nil }",
		"The user asked about database migrations and workspace config.",
		"key=value pairs like EDITOR=vim and HOME=/home/user are not secrets",
		"short=abc is too short to be a credential",
		"sk-1 is too short",
		"password=short",
		"memory_limit=512mb",
		"bearer abc is too short",
		"api_version=2023-10-16 is not a credential",
		"tokenize and tokenizer are ordinary words",
		`token: "bearer" is too short to redact`,
		`tokens = [1, 2, 3] is a list not a secret`,
		`{"max_tokens": 100} short values stay`,
		"Authorization: Basic dXNlcjpwYXNz", // basic-auth-ish but not covered shapes
	}
	for _, in := range cases {
		if got := Scrub(in); got != in {
			t.Errorf("Scrub changed non-secret text:\n in: %q\nout: %q", in, got)
		}
	}
}

func TestScrub_RedactsKnownShapes(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		marker  string
		leakage string
	}{
		{
			"openai sk- key",
			`const key = "sk-proj-AbCdEfGhIjKlMnOpQrStUvWxYz123456"`,
			"[REDACTED:api-key]",
			"sk-proj-AbCdEfGhIjKlMnOpQrStUvWxYz123456",
		},
		{
			"anthropic key",
			"ANTHROPIC_API_KEY=sk-ant-api03-AbCdEfGhIjKlMnOpQrStUv",
			"[REDACTED:api-key]",
			"sk-ant-api03-AbCdEfGhIjKlMnOpQrStUv",
		},
		{
			"github classic pat",
			"token: ghp_16C7e42F292c6912E7710c838347Ae178B4a",
			"[REDACTED:github-token]",
			"ghp_16C7e42F292c6912E7710c838347Ae178B4a",
		},
		{
			"github fine-grained pat",
			"github_pat_11ABCDEFG0A1B2c3d4E5F6_aBcDeFgHiJkLmNoPqRsTuVwXyZ",
			"[REDACTED:github-token]",
			"github_pat_11ABCDEFG0A1B2c3d4E5F6",
		},
		{
			"aws access key id",
			"aws_access_key_id = AKIAIOSFODNN7EXAMPLE",
			"[REDACTED:aws-key]",
			"AKIAIOSFODNN7EXAMPLE",
		},
		{
			"jwt",
			"Authorization: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			"[REDACTED:jwt]",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			"slack token",
			"xoxb-123456789012-abcdefghijkl",
			"[REDACTED:slack-token]",
			"xoxb-123456789012",
		},
		{
			"google api key",
			"AIzaSyD4iE2xVSpkLLOXoyq2uewu85bXaKlI2eY",
			"[REDACTED:google-key]",
			"AIzaSyD4iE2xVSpkLLOXoyq2uewu85bXaKlI2eY",
		},
		{
			"bearer token",
			"Authorization: Bearer abcdef0123456789abcdef0123",
			"[REDACTED:bearer]",
			"abcdef0123456789abcdef0123",
		},
		{
			"env password",
			"DB_PASSWORD=sup3r_secret_value_99",
			"[REDACTED]",
			"sup3r_secret_value_99",
		},
		{
			"env api key assignment keeps key name",
			"OPENAI_API_KEY=abcd1234efgh5678ijkl",
			"OPENAI_API_KEY=[REDACTED]",
			"abcd1234efgh5678ijkl",
		},
		{
			"npm token",
			"//registry.npmjs.org/:_authToken=npm_0123456789abcdefghijklmnopqrstuvwx",
			"[REDACTED:api-key]",
			"npm_0123456789abcdefghijklmnopqrstuvwx",
		},
		{
			"pem private key block",
			"-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA7\nfakebase64content\n-----END RSA PRIVATE KEY-----",
			"[REDACTED:private-key]",
			"fakebase64content",
		},
		{
			"openssh key",
			"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjE=\n-----END OPENSSH PRIVATE KEY-----",
			"[REDACTED:private-key]",
			"b3BlbnNzaC1rZXktdjE=",
		},
		{
			"client secret assignment",
			"client_secret: 'a1b2c3d4e5f6g7h8'",
			"[REDACTED]",
			"a1b2c3d4e5f6g7h8",
		},
		{
			"pgpass",
			"POSTGRES_PASSWORD=tr0ub4dor&3xtralong",
			"[REDACTED]",
			"tr0ub4dor",
		},
		{
			"json api_key pair",
			`{"api_key": "abcd1234efgh5678ijkl", "model": "x"}`,
			"[REDACTED]",
			"abcd1234efgh5678ijkl",
		},
		{
			"json password pair",
			`"password": "hunter2pass"`,
			"[REDACTED]",
			"hunter2pass",
		},
		{
			"js camelCase key",
			`apiKey: "zxcvbnm123456789"`,
			"[REDACTED]",
			"zxcvbnm123456789",
		},
		{
			"jwt secret env",
			"JWT_SECRET=supersecretjwtvalue123",
			"[REDACTED]",
			"supersecretjwtvalue123",
		},
		{
			"session secret env",
			"SESSION_SECRET=nextauthsecretvalue99",
			"[REDACTED]",
			"nextauthsecretvalue99",
		},
		{
			"github token env unshaped value",
			"GITHUB_TOKEN=unshapedtokenvalue12345",
			"[REDACTED]",
			"unshapedtokenvalue12345",
		},
		{
			"gitlab token env",
			"GITLAB_TOKEN=glpat-abcdefghijklmnop",
			"[REDACTED:api-key]",
			"glpat-abcdefghijklmnop",
		},
		{
			"stripe live key",
			"sk_live_51MAbCdEfGhIjKl",
			"[REDACTED:api-key]",
			"sk_live_51MAbCdEfGhIjKl",
		},
		{
			"huggingface token",
			"hf_abcdefghij1234567890klmnop",
			"[REDACTED:api-key]",
			"hf_abcdefghij1234567890klmnop",
		},
		{
			"slack app-level token",
			"xapp-1-abcdefghijklmnop",
			"[REDACTED:slack-token]",
			"xapp-1-abcdefghijklmnop",
		},
		{
			"quoted multi-word password",
			`password="my secret passphrase"`,
			"[REDACTED]",
			"secret passphrase",
		},
		{
			"quoted json token value",
			`"access_token": "ya29.a0AfH6SMBxabcdef123456"`,
			"[REDACTED]",
			"ya29.a0AfH6SMBxabcdef123456",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Scrub(tc.in)
			if strings.Contains(out, tc.leakage) {
				t.Errorf("secret still present in output: %q", out)
			}
			if !strings.Contains(out, tc.marker) {
				t.Errorf("expected marker %q in output: %q", tc.marker, out)
			}
		})
	}
}

func TestScrub_MultipleSecretsInOneText(t *testing.T) {
	in := "api_key=sk-AbCdEfGhIjKlMnOp1234 and also AKIAIOSFODNN7EXAMPLE plus password=hunter2hunter2"
	out := Scrub(in)
	if strings.Contains(out, "sk-AbCdEfGhIjKlMnOp1234") || strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") || strings.Contains(out, "hunter2hunter2") {
		t.Errorf("leaked secrets in output: %q", out)
	}
}
