package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type emptyImpactQuerier struct{}

func (emptyImpactQuerier) ImpactNodeExists(context.Context, sqlc.ImpactNodeExistsParams) (sql.NullBool, error) {
	return sql.NullBool{Bool: false, Valid: true}, nil
}

func (emptyImpactQuerier) GetImpactors(context.Context, sqlc.GetImpactorsParams) ([]sqlc.GetImpactorsRow, error) {
	return nil, nil
}

func (emptyImpactQuerier) GetImpactorsByTargets(context.Context, sqlc.GetImpactorsByTargetsParams) ([]sqlc.GetImpactorsByTargetsRow, error) {
	return nil, nil
}

func TestGraphImpact_EmptyResponseIsArray(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/impact",
		bytes.NewBufferString(`{"node":"missing.go::F"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "test-workspace")
	if err := handlers.GraphImpact(emptyImpactQuerier{}, zerolog.Nop())(c); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if string(got["impacted"]) != "[]" || string(got["node_found"]) != "false" {
		t.Fatalf("impact body = %s, want impacted:[] and node_found:false", rec.Body.String())
	}
}
