package search

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrInvalidCursor is returned when a cursor token is malformed
// (not valid base64url, not valid JSON, or contains a negative offset).
var ErrInvalidCursor = errors.New("invalid cursor")

// ErrCursorQueryMismatch is returned by VerifyCursor when the cursor's
// embedded query hash does not match the current request's query.
var ErrCursorQueryMismatch = errors.New("cursor query mismatch")

// MaxCursorOffset caps how deep a cursor may page. Bounds the SQL LIMIT
// derived from `offset + max_results + 1` so it cannot trigger int32
// overflow (#358 review) or unbounded server work. 10k offset × 100 per
// page = 1M results — well past any realistic agent workflow.
const MaxCursorOffset = 10_000

// cursorPayload is the internal JSON structure encoded in a cursor token.
type cursorPayload struct {
	Offset int    `json:"o"`
	QueryHash string `json:"q"`
}

// QueryHash returns the first 16 hex chars of sha256(query),
// used to guard against cross-query cursor reuse.
func QueryHash(query string) string {
	sum := sha256.Sum256([]byte(query))
	return fmt.Sprintf("%x", sum[:8])
}

// EncodeCursor encodes a pagination offset + query hash into an
// opaque base64url JSON token suitable for round-tripping through
// an MCP client. Format: base64url(JSON{"o":<offset>,"q":<hash>}).
// MUST NOT be parsed by clients.
func EncodeCursor(offset int, queryHash string) string {
	payload := cursorPayload{
		Offset:    offset,
		QueryHash: queryHash,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		// JSON marshaling of simple struct should never fail; panic if it does
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeCursor decodes a cursor token previously produced by EncodeCursor.
// Returns the embedded offset and queryHash, or an error wrapping one of:
//   - ErrInvalidCursor (base64 or JSON malformed, or negative offset)
func DecodeCursor(token string) (offset int, queryHash string, err error) {
	if token == "" {
		return 0, "", fmt.Errorf("%w: empty token", ErrInvalidCursor)
	}

	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %w", ErrInvalidCursor, err)
	}

	var p cursorPayload
	err = json.Unmarshal(data, &p)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %w", ErrInvalidCursor, err)
	}

	if p.Offset < 0 {
		return 0, "", fmt.Errorf("%w: negative offset %d", ErrInvalidCursor, p.Offset)
	}
	if p.Offset > MaxCursorOffset {
		return 0, "", fmt.Errorf("%w: offset %d exceeds maximum %d", ErrInvalidCursor, p.Offset, MaxCursorOffset)
	}

	return p.Offset, p.QueryHash, nil
}

// VerifyCursor decodes the token and confirms the embedded queryHash
// matches QueryHash(currentQuery). Returns the decoded offset on success.
// Returns ErrInvalidCursor for malformed cursors, ErrCursorQueryMismatch
// when the query has changed. Empty token is treated as first page (offset 0).
func VerifyCursor(token string, currentQuery string) (offset int, err error) {
	if token == "" {
		return 0, nil
	}

	decodedOffset, decodedHash, err := DecodeCursor(token)
	if err != nil {
		return 0, err
	}

	expectedHash := QueryHash(currentQuery)
	if decodedHash != expectedHash {
		return 0, fmt.Errorf("%w", ErrCursorQueryMismatch)
	}

	return decodedOffset, nil
}
