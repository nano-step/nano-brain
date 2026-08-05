package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type unresolvedTraceQuerier struct{}

func (unresolvedTraceQuerier) GetOutgoingEdges(_ context.Context, arg sqlc.GetOutgoingEdgesParams) ([]sqlc.GraphEdge, error) {
	if arg.SourceNode == "repo-a/consumer.ts::caller" {
		return []sqlc.GraphEdge{{
			SourceNode: arg.SourceNode,
			TargetNode: "<unresolved>",
			EdgeType:   "calls",
		}}, nil
	}
	return nil, nil
}

func (unresolvedTraceQuerier) GetOutgoingEdgesBySymbol(context.Context, sqlc.GetOutgoingEdgesBySymbolParams) ([]sqlc.GraphEdge, error) {
	return nil, nil
}

func TestGraphTraceExcludesUnresolvedFromDerivedResponse(t *testing.T) {
	// Given
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/trace", strings.NewReader(`{"node":"repo-a/consumer.ts::caller","max_depth":2}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "workspace")

	// When
	if err := handlers.GraphTrace(unresolvedTraceQuerier{}, zerolog.Nop())(c); err != nil {
		t.Fatalf("GraphTrace: %v", err)
	}

	// Then
	var body struct {
		Chain []json.RawMessage `json:"chain"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Chain) != 0 {
		t.Fatalf("unresolved sentinel entered derived trace response: %s", rec.Body.String())
	}
}
