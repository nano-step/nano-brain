//go:build integration

package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// TestTimeFilter_REST_ValidRelativeDuration tests §6.2: search with updated_after relative duration.
// Seed 3 documents at 5d, 20d, 60d ago. Filter with updated_after="30d" should return only docs
// updated within the last 30 days (5d and 20d).
func TestTimeFilter_REST_ValidRelativeDuration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	now := time.Now().UTC()
	doc5DaysAgo := now.AddDate(0, 0, -5)
	doc20DaysAgo := now.AddDate(0, 0, -20)
	doc60DaysAgo := now.AddDate(0, 0, -60)

	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-5d", "content updated 5 days ago", nil, doc5DaysAgo, doc5DaysAgo)
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-20d", "content updated 20 days ago", nil, doc20DaysAgo, doc20DaysAgo)
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-60d", "content updated 60 days ago", nil, doc60DaysAgo, doc60DaysAgo)

	e := echo.New()
	reqBody := map[string]interface{}{
		"query":          "content",
		"updated_after":  "30d",
		"max_results":    100,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("expected 2 results, got %d", resp.Total)
	}
	for _, r := range resp.Results {
		if r.SourcePath == "/tmp/doc-60d.md" {
			t.Errorf("doc-60d should be filtered out (older than 30d)")
		}
	}
}

// TestTimeFilter_REST_RFC3339CreatedAfter tests §6.3: search with created_after RFC3339 timestamp.
// Seed 3 docs with different created_at values. Filter with created_after=<30d-ago RFC3339>
// should return only doc created within the last 30 days.
func TestTimeFilter_REST_RFC3339CreatedAfter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	now := time.Now().UTC()
	createdOld := now.AddDate(0, 0, -60)
	createdMid := now.AddDate(0, 0, -40)
	createdRecent := now.AddDate(0, 0, -10)

	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-old", "old doc", nil, createdOld, now)
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-mid", "mid doc", nil, createdMid, now)
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-recent", "recent doc", nil, createdRecent, now)

	thirtyDaysAgo := now.AddDate(0, 0, -30)
	rfc3339Filter := thirtyDaysAgo.Format(time.RFC3339)

	e := echo.New()
	reqBody := map[string]interface{}{
		"query":           "doc",
		"created_after":   rfc3339Filter,
		"max_results":     100,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("expected 1 result (only recent doc), got %d", resp.Total)
	}
	if len(resp.Results) > 0 && resp.Results[0].SourcePath != "/tmp/doc-recent.md" {
		t.Errorf("expected only doc-recent, got %s", resp.Results[0].SourcePath)
	}
}

// TestTimeFilter_REST_AllFourFiltersCombined tests §6.4: all four filters with AND semantics.
func TestTimeFilter_REST_AllFourFiltersCombined(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	now := time.Now().UTC()
	docA := map[string]time.Time{
		"created_at": now.AddDate(0, 0, -50),
		"updated_at": now.AddDate(0, 0, -5),
	}
	docB := map[string]time.Time{
		"created_at": now.AddDate(0, 0, -100),
		"updated_at": now.AddDate(0, 0, -5),
	}
	docC := map[string]time.Time{
		"created_at": now.AddDate(0, 0, -30),
		"updated_at": now.AddDate(0, 0, -100),
	}

	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-a", "doc a", nil, docA["created_at"], docA["updated_at"])
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-b", "doc b", nil, docB["created_at"], docB["updated_at"])
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc-c", "doc c", nil, docC["created_at"], docC["updated_at"])

	createdAfter90d := now.AddDate(0, 0, -90).Format(time.RFC3339)
	createdBefore30d := now.AddDate(0, 0, -30).Format(time.RFC3339)
	updatedAfter30d := "30d"
	updatedBeforeNow := now.Format(time.RFC3339)

	e := echo.New()
	reqBody := map[string]interface{}{
		"query":            "doc",
		"created_after":    createdAfter90d,
		"created_before":   createdBefore30d,
		"updated_after":    updatedAfter30d,
		"updated_before":   updatedBeforeNow,
		"max_results":      100,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp handlers.SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("expected 1 result, got %d", resp.Total)
	}
	if len(resp.Results) > 0 && resp.Results[0].SourcePath != "/tmp/doc-a.md" {
		t.Errorf("expected doc-a, got %s", resp.Results[0].SourcePath)
	}
}

// TestTimeFilter_REST_InvalidDurationFormat tests §6.5: invalid duration returns HTTP 400
// with parameter name and value in error body.
func TestTimeFilter_REST_InvalidDurationFormat(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	e := echo.New()
	reqBody := map[string]interface{}{
		"query":          "test",
		"updated_after":  "banana",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if errResp["param"] != "updated_after" {
		t.Errorf("expected param 'updated_after', got %s", errResp["param"])
	}
	if errResp["value"] != "banana" {
		t.Errorf("expected value 'banana', got %s", errResp["value"])
	}
	if errResp["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

// TestTimeFilter_REST_DateOnlyRejected tests §6.6: date-only string returns HTTP 400.
func TestTimeFilter_REST_DateOnlyRejected(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	e := echo.New()
	reqBody := map[string]interface{}{
		"query":          "test",
		"updated_after":  "2026-05-04",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if errResp["param"] != "updated_after" {
		t.Errorf("expected param 'updated_after', got %s", errResp["param"])
	}
}

// TestTimeFilter_REST_NegativeDurationRejected tests §6.7: negative duration returns HTTP 400.
func TestTimeFilter_REST_NegativeDurationRejected(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	e := echo.New()
	reqBody := map[string]interface{}{
		"query":          "test",
		"updated_after":  "-30d",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if errResp["param"] != "updated_after" {
		t.Errorf("expected param 'updated_after', got %s", errResp["param"])
	}
}

// TestTimeFilter_REST_InvertedRangeReturnsEmpty tests §6.8: inverted range
// (updated_after > updated_before) returns 200 with empty results, NOT 400.
func TestTimeFilter_REST_InvertedRangeReturnsEmpty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	now := time.Now().UTC()
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc", "content", nil, now.AddDate(0, 0, -30), now.AddDate(0, 0, -30))

	e := echo.New()
	after := now.AddDate(0, 0, -10).Format(time.RFC3339)
	before := now.AddDate(0, 0, -20).Format(time.RFC3339)

	reqBody := map[string]interface{}{
		"query":          "content",
		"updated_after":  after,
		"updated_before": before,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("expected 0 results for inverted range, got %d", resp.Total)
	}
}

// TestTimeFilter_REST_NoMatchReturnsEmpty tests §6.9: filter matching zero documents
// returns 200 with empty results.
func TestTimeFilter_REST_NoMatchReturnsEmpty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	wsHash := "test_ws_" + uuid.New().String()[:8]
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash, Name: "test", Path: "/tmp/" + wsHash,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	now := time.Now().UTC()
	testutil.SeedDocumentWithTimestamps(t, ctx, db, wsHash, "doc", "content", nil, now.AddDate(0, -1, 0), now.AddDate(0, -1, 0))

	e := echo.New()
	reqBody := map[string]interface{}{
		"query":         "content",
		"updated_after": "7d",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", wsHash)

	handler := handlers.BM25Search(q, zerolog.Nop())
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("expected 0 results, got %d", resp.Total)
	}
}

// TestTimeFilter_REST_CursorInvalidationOnFilterChange tests §6.10:
// Pagination with time-range filter → cursor valid on same filter → cursor
// invalidated when time-range changes.
func TestTimeFilter_REST_CursorInvalidationOnFilterChange(t *testing.T) {
	now := time.Now().UTC()
	
	hashInput1 := search.QueryHashInput{
		Query:     "content",
		Tags:      []string{},
		Scope:     "test_ws",
		TimeRange: &search.TimeRangeFilter{
			UpdatedAfter:    &now,
			UpdatedAfterRaw: "60d",
		}, 
	}

	cursor1 := search.EncodeCursor(5, search.QueryHash(hashInput1))

	newUpdatedAfter := now.AddDate(0, 0, -7)
	hashInput2 := search.QueryHashInput{
		Query:     "content",
		Tags:      []string{},
		Scope:     "test_ws",
		TimeRange: &search.TimeRangeFilter{
			UpdatedAfter:    &newUpdatedAfter,
			UpdatedAfterRaw: "7d",
		},
	}

	offset2, cursorErr := search.VerifyCursor(cursor1, hashInput2)
	if cursorErr == nil {
		t.Errorf("expected cursor mismatch error when time-range changes, got none (offset=%d)", offset2)
	}
	if !errors.Is(cursorErr, search.ErrCursorQueryMismatch) {
		t.Errorf("expected ErrCursorQueryMismatch, got: %v", cursorErr)
	}
}

// TestTimeFilter_REST_CursorInvalidationOnTagsChange tests §6.11:
// Regression test for pre-existing bug: pagination with tags → cursor valid
// on same tags → cursor invalidated when tags change.
// Regression: pre-#360-task4, this scenario silently returned wrong results because
// QueryHash only included query text, not tags. Now QueryHash includes tags, so
// changing tags properly invalidates the cursor.
func TestTimeFilter_REST_CursorInvalidationOnTagsChange(t *testing.T) {
	tagsBug := []string{"bug"}
	hashInputBug := search.QueryHashInput{
		Query: "content",
		Tags:  tagsBug,
		Scope: "test_ws",
	}

	cursor1 := search.EncodeCursor(5, search.QueryHash(hashInputBug))

	tagsFeature := []string{"feature"}
	hashInputFeature := search.QueryHashInput{
		Query: "content",
		Tags:  tagsFeature,
		Scope: "test_ws",
	}

	offset2, cursorErr := search.VerifyCursor(cursor1, hashInputFeature)
	if cursorErr == nil {
		t.Errorf("expected cursor mismatch error when tags change, got none (offset=%d)", offset2)
	}
	if !errors.Is(cursorErr, search.ErrCursorQueryMismatch) {
		t.Errorf("expected ErrCursorQueryMismatch, got: %v", cursorErr)
	}
}
