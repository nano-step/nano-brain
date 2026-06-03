package search_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/search"
)

func TestQueryHash(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "same query produces same hash",
			query: "hello world",
		},
		{
			name:  "empty query",
			query: "",
		},
		{
			name:  "long query",
			query: "this is a much longer query with many words in it",
		},
		{
			name:  "unicode query",
			query: "你好世界",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := search.QueryHash(tt.query)
			hash2 := search.QueryHash(tt.query)

			// Same query must produce same hash (deterministic)
			if hash1 != hash2 {
				t.Errorf("QueryHash(%q) not deterministic: %s vs %s", tt.query, hash1, hash2)
			}

			// Hash must be exactly 16 hex characters
			if len(hash1) != 16 {
				t.Errorf("QueryHash(%q) length = %d, want 16, got %s", tt.query, len(hash1), hash1)
			}

			// Hash must only contain hex digits
			for _, c := range hash1 {
				if !strings.ContainsRune("0123456789abcdef", c) {
					t.Errorf("QueryHash(%q) contains non-hex character: %c in %s", tt.query, c, hash1)
				}
			}
		})
	}
}

func TestQueryHashDifferent(t *testing.T) {
	hash1 := search.QueryHash("query1")
	hash2 := search.QueryHash("query2")
	if hash1 == hash2 {
		t.Errorf("Different queries should produce different hashes: %s vs %s", hash1, hash2)
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	tests := []struct {
		name      string
		offset    int
		queryHash string
	}{
		{
			name:      "offset 0 with simple hash",
			offset:    0,
			queryHash: "abc123def456",
		},
		{
			name:      "offset 100 with QueryHash result",
			offset:    100,
			queryHash: search.QueryHash("hello world"),
		},
		{
			name:      "large offset (at MaxCursorOffset boundary)",
			offset:    9999,
			queryHash: search.QueryHash("test query"),
		},
		{
			name:      "offset 1",
			offset:    1,
			queryHash: "fedcba9876543210",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			cursor := search.EncodeCursor(tt.offset, tt.queryHash)

			// Cursor must be non-empty
			if cursor == "" {
				t.Errorf("EncodeCursor(%d, %q) returned empty string", tt.offset, tt.queryHash)
			}

			// Cursor must be URL-safe (no +, /, or =)
			if strings.ContainsAny(cursor, "+/=") {
				t.Errorf("EncodeCursor(%d, %q) contains non-URL-safe characters: %s", tt.offset, tt.queryHash, cursor)
			}

			// Decode
			decodedOffset, decodedHash, err := search.DecodeCursor(cursor)
			if err != nil {
				t.Fatalf("DecodeCursor(%q) returned error: %v", cursor, err)
			}

			// Verify roundtrip
			if decodedOffset != tt.offset {
				t.Errorf("DecodeCursor offset: got %d, want %d", decodedOffset, tt.offset)
			}
			if decodedHash != tt.queryHash {
				t.Errorf("DecodeCursor hash: got %s, want %s", decodedHash, tt.queryHash)
			}
		})
	}
}

func TestEncodeCursorDeterministic(t *testing.T) {
	// Same input must always produce same output
	offset := 42
	hash := search.QueryHash("test")

	cursor1 := search.EncodeCursor(offset, hash)
	cursor2 := search.EncodeCursor(offset, hash)

	if cursor1 != cursor2 {
		t.Errorf("EncodeCursor not deterministic: %s vs %s", cursor1, cursor2)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		shouldError bool
	}{
		{
			name:        "empty string",
			token:       "",
			shouldError: true,
		},
		{
			name:        "garbage",
			token:       "not-base64!@#",
			shouldError: true,
		},
		{
			name:        "valid base64 but invalid JSON",
			token:       base64.RawURLEncoding.EncodeToString([]byte("not json")),
			shouldError: true,
		},
		{
			name:        "negative offset",
			token:       encodeTestCursor(t, -1, "abc123"),
			shouldError: true,
		},
		{
			name:        "offset above MaxCursorOffset",
			token:       encodeTestCursor(t, search.MaxCursorOffset+1, "abc123"),
			shouldError: true,
		},
		{
			name:        "offset exactly at MaxCursorOffset is accepted",
			token:       encodeTestCursor(t, search.MaxCursorOffset, "abc123"),
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := search.DecodeCursor(tt.token)
			if !tt.shouldError && err != nil {
				t.Errorf("DecodeCursor(%q) returned unexpected error: %v", tt.token, err)
			}
			if tt.shouldError && err == nil {
				t.Errorf("DecodeCursor(%q) should have returned error", tt.token)
			}
			if tt.shouldError && err != nil && !errors.Is(err, search.ErrInvalidCursor) {
				t.Errorf("DecodeCursor(%q) error should wrap ErrInvalidCursor, got: %v", tt.token, err)
			}
		})
	}
}

func TestVerifyCursor(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		currentQuery  string
		wantOffset    int
		wantError     error
		shouldBeEmpty bool
	}{
		{
			name:         "empty token is first page",
			token:        "",
			currentQuery: "any query",
			wantOffset:   0,
			wantError:    nil,
		},
		{
			name:         "valid cursor matching query",
			token:        encodeCursorWithQuery(t, 50, "alpha"),
			currentQuery: "alpha",
			wantOffset:   50,
			wantError:    nil,
		},
		{
			name:         "valid cursor but different query",
			token:        encodeCursorWithQuery(t, 50, "alpha"),
			currentQuery: "beta",
			wantOffset:   0, // offset not used on mismatch
			wantError:    search.ErrCursorQueryMismatch,
		},
		{
			name:         "invalid cursor with garbage",
			token:        "not-valid-cursor",
			currentQuery: "any query",
			wantOffset:   0,
			wantError:    search.ErrInvalidCursor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, err := search.VerifyCursor(tt.token, tt.currentQuery)

			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					t.Errorf("VerifyCursor(%q, %q) error: got %v, want %v", tt.token, tt.currentQuery, err, tt.wantError)
				}
			} else if err != nil {
				t.Errorf("VerifyCursor(%q, %q) returned unexpected error: %v", tt.token, tt.currentQuery, err)
			}

			if tt.wantError == nil && offset != tt.wantOffset {
				t.Errorf("VerifyCursor(%q, %q) offset: got %d, want %d", tt.token, tt.currentQuery, offset, tt.wantOffset)
			}
		})
	}
}

// Helper: encode a cursor with a specific query for testing
func encodeCursorWithQuery(t *testing.T, offset int, query string) string {
	hash := search.QueryHash(query)
	return search.EncodeCursor(offset, hash)
}

// Helper: encode a raw cursor payload with specific offset (for negative offset test)
func encodeTestCursor(t *testing.T, offset int, hash string) string {
	payload := map[string]interface{}{
		"o": offset,
		"q": hash,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal test cursor: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
