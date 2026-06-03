package search_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/search"
)

func TestQueryHashInput(t *testing.T) {
	tests := []struct {
		name  string
		input search.QueryHashInput
	}{
		{
			name: "(a) same query + same all-filters → same hash",
			input: search.QueryHashInput{
				Query:       "hello world",
				Tags:        []string{},
				Scope:       "ws1",
				Collections: []string{},
				TimeRange:   nil,
			},
		},
		{
			name: "(b) query change → different hash",
			input: search.QueryHashInput{
				Query:       "different query",
				Tags:        []string{},
				Scope:       "ws1",
				Collections: []string{},
				TimeRange:   nil,
			},
		},
		{
			name: "(c) tag added → different hash (pre-existing bug fix)",
			input: search.QueryHashInput{
				Query:       "hello world",
				Tags:        []string{"tag1"},
				Scope:       "ws1",
				Collections: []string{},
				TimeRange:   nil,
			},
		},
		{
			name: "(e) collections change → different hash",
			input: search.QueryHashInput{
				Query:       "hello world",
				Tags:        []string{},
				Scope:       "ws1",
				Collections: []string{"col1"},
				TimeRange:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := search.QueryHash(tt.input)
			hash2 := search.QueryHash(tt.input)

			if hash1 != hash2 {
				t.Errorf("QueryHash not deterministic: %s vs %s", hash1, hash2)
			}

			if len(hash1) != 16 {
				t.Errorf("QueryHash length = %d, want 16", len(hash1))
			}

			for _, c := range hash1 {
				if !strings.ContainsRune("0123456789abcdef", c) {
					t.Errorf("QueryHash contains non-hex character: %c", c)
				}
			}
		})
	}
}

func TestQueryHashDifferent(t *testing.T) {
	tests := []struct {
		name  string
		inp1  search.QueryHashInput
		inp2  search.QueryHashInput
		descr string
	}{
		{
			name: "query change",
			inp1: search.QueryHashInput{Query: "query1", Scope: "ws1"},
			inp2: search.QueryHashInput{Query: "query2", Scope: "ws1"},
			descr: "Different queries",
		},
		{
			name: "tag change",
			inp1: search.QueryHashInput{Query: "q", Tags: []string{"tag1"}, Scope: "ws1"},
			inp2: search.QueryHashInput{Query: "q", Tags: []string{"tag2"}, Scope: "ws1"},
			descr: "Different tags",
		},
		{
			name: "scope change",
			inp1: search.QueryHashInput{Query: "q", Scope: "ws1"},
			inp2: search.QueryHashInput{Query: "q", Scope: "ws2"},
			descr: "Different scopes",
		},
		{
			name: "collections change",
			inp1: search.QueryHashInput{Query: "q", Collections: []string{"c1"}, Scope: "ws1"},
			inp2: search.QueryHashInput{Query: "q", Collections: []string{"c2"}, Scope: "ws1"},
			descr: "Different collections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := search.QueryHash(tt.inp1)
			hash2 := search.QueryHash(tt.inp2)
			if hash1 == hash2 {
				t.Errorf("%s should produce different hashes: %s vs %s", tt.descr, hash1, hash2)
			}
		})
	}
}

func TestQueryHashTagOrder(t *testing.T) {
	inp1 := search.QueryHashInput{
		Query: "q",
		Tags:  []string{"a", "b", "c"},
		Scope: "ws1",
	}
	inp2 := search.QueryHashInput{
		Query: "q",
		Tags:  []string{"c", "a", "b"},
		Scope: "ws1",
	}

	hash1 := search.QueryHash(inp1)
	hash2 := search.QueryHash(inp2)
	if hash1 != hash2 {
		t.Errorf("(d) tag order rearranged but same set: hashes should match: %s vs %s", hash1, hash2)
	}
}

func TestQueryHashCollectionOrder(t *testing.T) {
	inp1 := search.QueryHashInput{
		Query:       "q",
		Collections: []string{"x", "y", "z"},
		Scope:       "ws1",
	}
	inp2 := search.QueryHashInput{
		Query:       "q",
		Collections: []string{"z", "x", "y"},
		Scope:       "ws1",
	}

	hash1 := search.QueryHash(inp1)
	hash2 := search.QueryHash(inp2)
	if hash1 != hash2 {
		t.Errorf("collections order rearranged but same set: hashes should match: %s vs %s", hash1, hash2)
	}
}

func TestQueryHashTimeRangeRawStrings(t *testing.T) {
	tr1 := &search.TimeRangeFilter{
		UpdatedAfterRaw: "30d",
	}
	tr2 := &search.TimeRangeFilter{
		UpdatedAfterRaw: "30d",
	}

	inp1 := search.QueryHashInput{
		Query:     "q",
		Scope:     "ws1",
		TimeRange: tr1,
	}
	inp2 := search.QueryHashInput{
		Query:     "q",
		Scope:     "ws1",
		TimeRange: tr2,
	}

	hash1 := search.QueryHash(inp1)
	hash2 := search.QueryHash(inp2)
	if hash1 != hash2 {
		t.Errorf("(g) same time-range raw strings should produce same hash: %s vs %s", hash1, hash2)
	}
}

func TestQueryHashTimeRangeChange(t *testing.T) {
	tr1 := &search.TimeRangeFilter{
		UpdatedAfterRaw: "30d",
	}
	tr2 := &search.TimeRangeFilter{
		UpdatedAfterRaw: "60d",
	}

	inp1 := search.QueryHashInput{
		Query:     "q",
		Scope:     "ws1",
		TimeRange: tr1,
	}
	inp2 := search.QueryHashInput{
		Query:     "q",
		Scope:     "ws1",
		TimeRange: tr2,
	}

	hash1 := search.QueryHash(inp1)
	hash2 := search.QueryHash(inp2)
	if hash1 == hash2 {
		t.Errorf("(f) different time-range raw strings should produce different hashes: %s vs %s", hash1, hash2)
	}
}

func TestQueryHashNilTimeRange(t *testing.T) {
	inp1 := search.QueryHashInput{
		Query:     "q",
		Scope:     "ws1",
		TimeRange: nil,
	}
	inp2 := search.QueryHashInput{
		Query:     "q",
		Scope:     "ws1",
		TimeRange: &search.TimeRangeFilter{},
	}

	hash1 := search.QueryHash(inp1)
	hash2 := search.QueryHash(inp2)
	if hash1 != hash2 {
		t.Errorf("(i) nil and empty TimeRangeFilter should produce same hash: %s vs %s", hash1, hash2)
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
			name: "offset 100 with QueryHash result",
			offset: 100,
			queryHash: search.QueryHash(search.QueryHashInput{
				Query: "hello world",
				Scope: "ws1",
			}),
		},
		{
			name: "large offset (at MaxCursorOffset boundary)",
			offset: 9999,
			queryHash: search.QueryHash(search.QueryHashInput{
				Query: "test query",
				Scope: "ws1",
			}),
		},
		{
			name:      "offset 1",
			offset:    1,
			queryHash: "fedcba9876543210",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor := search.EncodeCursor(tt.offset, tt.queryHash)

			if cursor == "" {
				t.Errorf("EncodeCursor(%d, %q) returned empty string", tt.offset, tt.queryHash)
			}

			if strings.ContainsAny(cursor, "+/=") {
				t.Errorf("EncodeCursor(%d, %q) contains non-URL-safe characters: %s", tt.offset, tt.queryHash, cursor)
			}

			decodedOffset, decodedHash, err := search.DecodeCursor(cursor)
			if err != nil {
				t.Fatalf("DecodeCursor(%q) returned error: %v", cursor, err)
			}

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
	offset := 42
	input := search.QueryHashInput{Query: "test", Scope: "ws1"}
	hash := search.QueryHash(input)

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
		name      string
		token     string
		input     search.QueryHashInput
		wantOffset int
		wantError  error
	}{
		{
			name:       "empty token is first page",
			token:      "",
			input:      search.QueryHashInput{Query: "any query", Scope: "ws1"},
			wantOffset: 0,
			wantError:  nil,
		},
		{
			name:       "valid cursor matching query",
			token:      encodeCursorWithInput(t, 50, search.QueryHashInput{Query: "alpha", Scope: "ws1"}),
			input:      search.QueryHashInput{Query: "alpha", Scope: "ws1"},
			wantOffset: 50,
			wantError:  nil,
		},
		{
			name:       "valid cursor but different query",
			token:      encodeCursorWithInput(t, 50, search.QueryHashInput{Query: "alpha", Scope: "ws1"}),
			input:      search.QueryHashInput{Query: "beta", Scope: "ws1"},
			wantOffset: 0,
			wantError:  search.ErrCursorQueryMismatch,
		},
		{
			name:       "valid cursor but different tags (pre-existing bug)",
			token:      encodeCursorWithInput(t, 10, search.QueryHashInput{Query: "q", Tags: []string{"tag1"}, Scope: "ws1"}),
			input:      search.QueryHashInput{Query: "q", Tags: []string{"tag2"}, Scope: "ws1"},
			wantOffset: 0,
			wantError:  search.ErrCursorQueryMismatch,
		},
		{
			name:       "valid cursor but different scope",
			token:      encodeCursorWithInput(t, 10, search.QueryHashInput{Query: "q", Scope: "ws1"}),
			input:      search.QueryHashInput{Query: "q", Scope: "ws2"},
			wantOffset: 0,
			wantError:  search.ErrCursorQueryMismatch,
		},
		{
			name:       "invalid cursor with garbage",
			token:      "not-valid-cursor",
			input:      search.QueryHashInput{Query: "any query", Scope: "ws1"},
			wantOffset: 0,
			wantError:  search.ErrInvalidCursor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, err := search.VerifyCursor(tt.token, tt.input)

			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					t.Errorf("VerifyCursor error: got %v, want %v", err, tt.wantError)
				}
			} else if err != nil {
				t.Errorf("VerifyCursor returned unexpected error: %v", err)
			}

			if tt.wantError == nil && offset != tt.wantOffset {
				t.Errorf("VerifyCursor offset: got %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}

func encodeCursorWithInput(t *testing.T, offset int, input search.QueryHashInput) string {
	hash := search.QueryHash(input)
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

func TestTimeRangeFilter_ToSqlNullTimes_NilReceiver(t *testing.T) {
	// Verify that nil receiver doesn't panic and returns four sql.NullTime{Valid: false}
	var f *search.TimeRangeFilter
	ca, cb, ua, ub := f.ToSqlNullTimes()

	if ca.Valid || cb.Valid || ua.Valid || ub.Valid {
		t.Errorf("Expected all sql.NullTime{Valid: false}, got ca.Valid=%v, cb.Valid=%v, ua.Valid=%v, ub.Valid=%v",
			ca.Valid, cb.Valid, ua.Valid, ub.Valid)
	}
}
