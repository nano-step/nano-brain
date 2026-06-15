package graph_test

import (
	"os"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func newNestJSExtractor(t *testing.T) *graph.NestJSExtractor {
	t.Helper()
	logger := zerolog.Nop()
	ex, err := graph.NewNestJSExtractor(logger)
	if err != nil {
		t.Fatalf("NewNestJSExtractor: %v", err)
	}
	return ex
}

func TestNestJSExtractor_Supports(t *testing.T) {
	ex := newNestJSExtractor(t)
	for _, ext := range []string{".ts", ".tsx"} {
		if !ex.Supports(ext) {
			t.Errorf("should support %q", ext)
		}
	}
	for _, ext := range []string{".js", ".jsx", ".py", ".go", ".rb", ".java", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

func TestNestJSExtractor_RequiresFrameworks(t *testing.T) {
	ex := newNestJSExtractor(t)
	fws := ex.RequiresFrameworks()
	if len(fws) != 1 || fws[0] != "nestjs" {
		t.Errorf("expected [nestjs], got %v", fws)
	}
}

func TestNestJSExtractor_BasicController(t *testing.T) {
	src := []byte(`import { Controller, Get, Post, Put, Delete, Patch } from '@nestjs/common';

@Controller('users')
export class UsersController {
  @Get()
  findAll() {}

  @Get(':id')
  findById(@Param('id') id: string) {}

  @Post()
  create(@Body() body: any) {}

  @Put(':id')
  update(@Param('id') id: string, @Body() body: any) {}

  @Delete(':id')
  remove(@Param('id') id: string) {}

  @Patch(':id')
  patch(@Param('id') id: string, @Body() body: any) {}
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("users.controller.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct{ verb, path, handler string }{
		{"GET", "/users", "UsersController.findAll"},
		{"GET", "/users/:id", "UsersController.findById"},
		{"POST", "/users", "UsersController.create"},
		{"PUT", "/users/:id", "UsersController.update"},
		{"DELETE", "/users/:id", "UsersController.remove"},
		{"PATCH", "/users/:id", "UsersController.patch"},
	}
	for _, tc := range cases {
		entryNode := tc.verb + " " + tc.path
		hits := findEdges(edges, graph.EdgeHTTP, entryNode, tc.handler)
		if len(hits) == 0 {
			t.Errorf("missing http edge %s -> %s", entryNode, tc.handler)
		}
	}
}

func TestNestJSExtractor_ControllerWithoutPrefix(t *testing.T) {
	src := []byte(`import { Controller, Get } from '@nestjs/common';

@Controller()
export class AppController {
  @Get()
  root() {
    return 'ok';
  }

  @Get('health')
  health() {
    return { status: 'ok' };
  }
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("app.controller.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /", "AppController.root")
	if len(hits) == 0 {
		t.Errorf("expected http edge GET / -> AppController.root; got %+v", edges)
	}

	hits = findEdges(edges, graph.EdgeHTTP, "GET /health", "AppController.health")
	if len(hits) == 0 {
		t.Errorf("expected http edge GET /health -> AppController.health; got %+v", edges)
	}
}

func TestNestJSExtractor_AllHTTPMethods(t *testing.T) {
	src := []byte(`import { Controller, Get, Post, Put, Delete, Patch, Head, Options, All } from '@nestjs/common';

@Controller('api')
export class ApiController {
  @Get()
  get() {}
  @Post()
  post() {}
  @Put()
  put() {}
  @Delete()
  del() {}
  @Patch()
  patch() {}
  @Head()
  head() {}
  @Options()
  options() {}
  @All()
  all() {}
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("api.controller.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	expectedHandlers := []struct{ method, handler string }{
		{"GET", "ApiController.get"},
		{"POST", "ApiController.post"},
		{"PUT", "ApiController.put"},
		{"DELETE", "ApiController.del"},
		{"PATCH", "ApiController.patch"},
		{"HEAD", "ApiController.head"},
		{"OPTIONS", "ApiController.options"},
		{"ALL", "ApiController.all"},
	}
	for _, tc := range expectedHandlers {
		entryNode := tc.method + " /api"
		hits := findEdges(edges, graph.EdgeHTTP, entryNode, tc.handler)
		if len(hits) == 0 {
			t.Errorf("missing http edge %s -> %s", entryNode, tc.handler)
		}
	}
}

func TestNestJSExtractor_EmptyFile(t *testing.T) {
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("empty.ts", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from empty file, got %+v", edges)
	}
}

func TestNestJSExtractor_NonNestJSFile(t *testing.T) {
	src := []byte(`import React from 'react';

function App() {
  return <div>Hello</div>;
}

export default App;
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("App.tsx", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from React file, got %+v", edges)
	}
}

func TestNestJSExtractor_LineField(t *testing.T) {
	src := []byte(`import { Controller, Get } from '@nestjs/common';

@Controller('items')
export class ItemsController {
  @Get()
  findAll() {}
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("items.controller.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /items", "ItemsController.findAll")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if hits[0].Line == 0 {
		t.Errorf("expected non-zero Line field; got 0")
	}
}

func TestNestJSExtractor_MetadataField(t *testing.T) {
	src := []byte(`import { Controller, Get } from '@nestjs/common';

@Controller('items')
export class ItemsController {
  @Get()
  findAll() {}
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("items.controller.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /items", "ItemsController.findAll")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if hits[0].Metadata == nil {
		t.Fatalf("expected non-nil Metadata; got nil")
	}
	if hits[0].Metadata["method"] != "GET" {
		t.Errorf("expected Metadata method=GET, got %v", hits[0].Metadata["method"])
	}
	if hits[0].Metadata["path"] != "/items" {
		t.Errorf("expected Metadata path=/items, got %v", hits[0].Metadata["path"])
	}
}

func TestNestJSExtractor_SourceFileNormalized(t *testing.T) {
	src := []byte(`import { Controller, Get } from '@nestjs/common';

@Controller('items')
export class ItemsController {
  @Get('search')
  search() {}
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("src/items.controller.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /items/search", "ItemsController.search")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if hits[0].SourceFile != "src/items.controller.ts" {
		t.Errorf("expected SourceFile 'src/items.controller.ts', got %q", hits[0].SourceFile)
	}
}

func TestNestJSExtractor_WithFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/nestjs/users.controller.ts")
	if err != nil {
		t.Fatal(err)
	}
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("testdata/nestjs/users.controller.ts", data)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct{ verb, path, handler string }{
		{"GET", "/users", "UsersController.findAll"},
		{"GET", "/users/:id", "UsersController.findById"},
		{"POST", "/users", "UsersController.create"},
	}
	for _, tc := range cases {
		entryNode := tc.verb + " " + tc.path
		hits := findEdges(edges, graph.EdgeHTTP, entryNode, tc.handler)
		if len(hits) == 0 {
			t.Errorf("missing http edge %s -> %s", entryNode, tc.handler)
		}
	}
}

func TestNestJSExtractor_NoControllerFile(t *testing.T) {
	src := []byte(`import { Injectable } from '@nestjs/common';

@Injectable()
export class UsersService {
  findAll() { return []; }
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("users.service.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from non-controller file, got %+v", edges)
	}
}

func TestNestJSExtractor_NestedPathController(t *testing.T) {
	src := []byte(`import { Controller, Get, Post } from '@nestjs/common';

@Controller('api/v1/users')
export class UserController {
  @Get()
  findAll() {}

  @Post()
  create(@Body() body: any) {}
}
`)
	ex := newNestJSExtractor(t)
	edges, err := ex.ExtractEdges("user.controller.ts", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /api/v1/users", "UserController.findAll")
	if len(hits) == 0 {
		t.Errorf("expected http edge GET /api/v1/users -> UserController.findAll; got %+v", edges)
	}
	hits = findEdges(edges, graph.EdgeHTTP, "POST /api/v1/users", "UserController.create")
	if len(hits) == 0 {
		t.Errorf("expected http edge POST /api/v1/users -> UserController.create; got %+v", edges)
	}
}

