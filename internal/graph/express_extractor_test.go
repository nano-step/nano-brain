package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func newExpressExtractor(t *testing.T) *graph.ExpressExtractor {
	t.Helper()
	logger := zerolog.Nop()
	ex, err := graph.NewExpressExtractor(logger)
	if err != nil {
		t.Fatalf("NewExpressExtractor: %v", err)
	}
	return ex
}

func TestExpressExtractor_Supports(t *testing.T) {
	ex := newExpressExtractor(t)
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx"} {
		if !ex.Supports(ext) {
			t.Errorf("should support %q", ext)
		}
	}
	for _, ext := range []string{".py", ".go", ".rb", ".java", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

func TestExpressExtractor_SimpleRoutes(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/users', listUsers);
app.post('/users', createUser);
app.put('/users/:id', updateUser);
app.delete('/users/:id', deleteUser);
app.patch('/users/:id', patchUser);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct{ verb, path, handler string }{
		{"GET", "/users", "listUsers"},
		{"POST", "/users", "createUser"},
		{"PUT", "/users/:id", "updateUser"},
		{"DELETE", "/users/:id", "deleteUser"},
		{"PATCH", "/users/:id", "patchUser"},
	}
	for _, tc := range cases {
		entryNode := tc.verb + " " + tc.path
		hits := findEdges(edges, graph.EdgeHTTP, entryNode, tc.handler)
		if len(hits) == 0 {
			t.Errorf("missing http edge %s → %s; edges: %+v", entryNode, tc.handler, edges)
		}
	}
}

func TestExpressExtractor_RouterRoutes(t *testing.T) {
	src := []byte(`import express from 'express';
const router = express.Router();

router.get('/posts', listPosts);
router.post('/posts', createPost);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits1 := findEdges(edges, graph.EdgeHTTP, "GET /posts", "listPosts")
	if len(hits1) == 0 {
		t.Fatalf("expected http edge GET /posts → listPosts; got %+v", edges)
	}
	hits2 := findEdges(edges, graph.EdgeHTTP, "POST /posts", "createPost")
	if len(hits2) == 0 {
		t.Fatalf("expected http edge POST /posts → createPost; got %+v", edges)
	}
}

func TestExpressExtractor_MemberExpressionHandler(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/users', userController.list);
app.post('/users', userController.create);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits1 := findEdges(edges, graph.EdgeHTTP, "GET /users", "userController.list")
	if len(hits1) == 0 {
		t.Fatalf("expected http edge GET /users → userController.list; got %+v", edges)
	}
	hits2 := findEdges(edges, graph.EdgeHTTP, "POST /users", "userController.create")
	if len(hits2) == 0 {
		t.Fatalf("expected http edge POST /users → userController.create; got %+v", edges)
	}
}

func TestExpressExtractor_AnonymousHandler(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/health', () => { return 'ok'; });
app.post('/data', (req, res) => { res.send('done'); });
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	httpEdges := findEdges(edges, graph.EdgeHTTP, "GET /health", "")
	if len(httpEdges) == 0 {
		t.Fatalf("expected http edge for anonymous handler; got %+v", edges)
	}
	if httpEdges[0].TargetNode == "" {
		t.Errorf("TargetNode should not be empty for anonymous handler")
	}
}

func TestExpressExtractor_MiddlewareExtraction(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.use('/api', authMiddleware, handleRequest);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	httpEdges := findEdges(edges, graph.EdgeHTTP, "USE /api", "handleRequest")
	if len(httpEdges) == 0 {
		t.Fatalf("expected http edge USE /api → handleRequest; got %+v", edges)
	}

	mwEdges := findEdges(edges, graph.EdgeMiddleware, "authMiddleware", "handleRequest")
	if len(mwEdges) == 0 {
		t.Fatalf("expected middleware edge authMiddleware → handleRequest; got %+v", edges)
	}
}

func TestExpressExtractor_RouteMiddleware(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/admin', requireAdmin, handleAdmin);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	mwEdges := findEdges(edges, graph.EdgeMiddleware, "requireAdmin", "handleAdmin")
	if len(mwEdges) == 0 {
		t.Fatalf("expected middleware edge requireAdmin → handleAdmin; got %+v", edges)
	}
}

func TestExpressExtractor_EmptyFile(t *testing.T) {
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("empty.ts", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from empty file, got %+v", edges)
	}
}

func TestExpressExtractor_NonExpressFile(t *testing.T) {
	src := []byte(`import React from 'react';

function App() {
  return <div>Hello</div>;
}

export default App;
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("App.tsx", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from React file, got %+v", edges)
	}
}

func TestExpressExtractor_MultipleRoutes(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/api/v1/users', listUsersV1);
app.get('/api/v2/users', listUsersV2);
app.post('/api/v1/users', createUserV1);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /api/v1/users", "listUsersV1")
	if len(hits) == 0 {
		t.Errorf("expected http edge GET /api/v1/users → listUsersV1; got %+v", edges)
	}
	hits = findEdges(edges, graph.EdgeHTTP, "GET /api/v2/users", "listUsersV2")
	if len(hits) == 0 {
		t.Errorf("expected http edge GET /api/v2/users → listUsersV2; got %+v", edges)
	}
	hits = findEdges(edges, graph.EdgeHTTP, "POST /api/v1/users", "createUserV1")
	if len(hits) == 0 {
		t.Errorf("expected http edge POST /api/v1/users → createUserV1; got %+v", edges)
	}
}

func TestExpressExtractor_JavaScriptFile(t *testing.T) {
	src := []byte(`const express = require('express');
const app = express();

app.get('/js-route', handleJsRoute);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.js", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /js-route", "handleJsRoute")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /js-route → handleJsRoute; got %+v", edges)
	}
	if hits[0].Language != "javascript" {
		t.Errorf("expected Language 'javascript' for .js file, got %q", hits[0].Language)
	}
}

func TestExpressExtractor_LineField(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/users', listUsers);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /users", "listUsers")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if hits[0].Line == 0 {
		t.Errorf("expected non-zero Line field; got 0")
	}
}

func TestExpressExtractor_MetadataField(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/users', listUsers);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /users", "listUsers")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if hits[0].Metadata == nil {
		t.Fatalf("expected non-nil Metadata; got nil")
	}
	if hits[0].Metadata["method"] != "GET" {
		t.Errorf("expected Metadata method=GET, got %v", hits[0].Metadata["method"])
	}
	if hits[0].Metadata["path"] != "/users" {
		t.Errorf("expected Metadata path=/users, got %v", hits[0].Metadata["path"])
	}
}

func TestExpressExtractor_SourceFileNormalized(t *testing.T) {
	src := []byte(`import express from 'express';
const app = express();

app.get('/users', listUsers);
`)
	ex := newExpressExtractor(t)
	edges, err := ex.ExtractEdges("src/routes.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /users", "listUsers")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if hits[0].SourceFile != "src/routes.ts" {
		t.Errorf("expected SourceFile 'src/routes.ts', got %q", hits[0].SourceFile)
	}
}
