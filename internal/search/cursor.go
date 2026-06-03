package search

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TimeRangeFilter carries optional document-timestamp bounds for search calls.
// Zero-value struct (all nil) is a valid "no filters" sentinel.
type TimeRangeFilter struct {
	// Parsed absolute cutoffs. nil = filter omitted.
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time

	// Original raw input strings, preserved for cursor-hash stability across
	// paginated calls with relative durations like "30d" (per design D5 / R1).
	// Empty string = filter omitted.
	CreatedAfterRaw  string
	CreatedBeforeRaw string
	UpdatedAfterRaw  string
	UpdatedBeforeRaw string
}

// ToSqlNullTimes converts the parsed time.Time fields to sql.NullTime.
// Returns (CreatedAfter, CreatedBefore, UpdatedAfter, UpdatedBefore) in that order.
// nil *time.Time becomes sql.NullTime{Valid: false}.
func (f *TimeRangeFilter) ToSqlNullTimes() (ca, cb, ua, ub sql.NullTime) {
	if f == nil {
		return ca, cb, ua, ub // all zero-value (Valid: false)
	}
	if f.CreatedAfter != nil {
		ca = sql.NullTime{Time: *f.CreatedAfter, Valid: true}
	}
	if f.CreatedBefore != nil {
		cb = sql.NullTime{Time: *f.CreatedBefore, Valid: true}
	}
	if f.UpdatedAfter != nil {
		ua = sql.NullTime{Time: *f.UpdatedAfter, Valid: true}
	}
	if f.UpdatedBefore != nil {
		ub = sql.NullTime{Time: *f.UpdatedBefore, Valid: true}
	}
	return ca, cb, ua, ub
}

// CursorString returns a deterministic representation of the raw time-range inputs
// for cursor hashing. Uses raw input strings (NOT parsed times) to preserve stability
// across paginated calls when relative durations like "30d" are used.
// Empty strings are represented as the literal "null".
func (f *TimeRangeFilter) CursorString() string {
	if f == nil {
		return "null|null|null|null"
	}
	emptyOr := func(s string) string {
		if s == "" {
			return "null"
		}
		return s
	}
	return fmt.Sprintf("%s|%s|%s|%s",
		emptyOr(f.CreatedAfterRaw),
		emptyOr(f.CreatedBeforeRaw),
		emptyOr(f.UpdatedAfterRaw),
		emptyOr(f.UpdatedBeforeRaw),
	)
}

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

// QueryHashInput encapsulates all filter components used for cursor hash computation.
// This ensures that cursors invalidate when ANY filter changes, not just the query text.
type QueryHashInput struct {
	Query       string
	Tags        []string
	Scope       string
	Collections []string
	TimeRange   *TimeRangeFilter
}

// QueryHash returns the first 16 hex chars of sha256(...),
// used to guard against cross-query cursor reuse.
// Hashes ALL filter inputs: query text + tags + scope + collections + time-range raw inputs.
// Raw time-range strings (not parsed times) are used to preserve cursor stability across
// paginated calls when relative durations like "30d" are used.
func QueryHash(input QueryHashInput) string {
	var components []string

	components = append(components, input.Query)

	sortedTags := make([]string, len(input.Tags))
	copy(sortedTags, input.Tags)
	sort.Strings(sortedTags)
	components = append(components, strings.Join(sortedTags, ","))

	components = append(components, input.Scope)

	sortedCollections := make([]string, len(input.Collections))
	copy(sortedCollections, input.Collections)
	sort.Strings(sortedCollections)
	components = append(components, strings.Join(sortedCollections, ","))

	timeRangeStr := "null|null|null|null"
	if input.TimeRange != nil {
		timeRangeStr = input.TimeRange.CursorString()
	}
	components = append(components, timeRangeStr)

	combined := strings.Join(components, "\x1f")

	sum := sha256.Sum256([]byte(combined))
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
// matches QueryHash(input). Returns the decoded offset on success.
// Returns ErrInvalidCursor for malformed cursors, ErrCursorQueryMismatch
// when any filter has changed. Empty token is treated as first page (offset 0).
func VerifyCursor(token string, input QueryHashInput) (offset int, err error) {
	if token == "" {
		return 0, nil
	}

	decodedOffset, decodedHash, err := DecodeCursor(token)
	if err != nil {
		return 0, err
	}

	expectedHash := QueryHash(input)
	if decodedHash != expectedHash {
		return 0, fmt.Errorf("%w", ErrCursorQueryMismatch)
	}

	return decodedOffset, nil
}
