package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newDetector(t *testing.T) *graph.FrameworkDetector {
	t.Helper()
	return graph.NewFrameworkDetector(graph.DefaultRules)
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func createTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "detector-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestDetect_GoModWithEcho(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "go.mod", `module test

go 1.23

require github.com/labstack/echo/v4 v4.12.0
`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "echo", "go")
}

func TestDetect_GoModWithGin(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "go.mod", `module test

go 1.23

require github.com/gin-gonic/gin v1.10.0
`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "gin", "go")
}

func TestDetect_GoModWithBoth(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "go.mod", `module test

go 1.23

require (
	github.com/labstack/echo/v4 v4.12.0
	github.com/gin-gonic/gin v1.10.0
)
`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "echo", "gin", "go")
}

func TestDetect_GoModWithoutFrameworks(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "go.mod", `module test

go 1.23
`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "go")
	if hasFramework(fws, "echo") || hasFramework(fws, "gin") || hasFramework(fws, "express") {
		t.Errorf("expected only 'go', got %v", fws)
	}
}

func TestDetect_PackageJSONWithExpress(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "package.json", `{
	"dependencies": {
		"express": "^4.18.0"
	}
}`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "express")
}

func TestDetect_PackageJSONExpressDevDeps(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "package.json", `{
	"devDependencies": {
		"express": "^4.18.0"
	}
}`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "express")
}

func TestDetect_NoManifests(t *testing.T) {
	dir := createTempDir(t)
	d := newDetector(t)
	fws := d.Detect(dir)
	if len(fws) != 0 {
		t.Errorf("expected empty, got %v", fws)
	}
}

func TestDetect_GemfileWithRails(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "Gemfile", `source "https://rubygems.org"

gem "rails", "~> 7.1"
`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "rails")
}

func TestDetect_GemfileRailsSingleQuotes(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "Gemfile", `source 'https://rubygems.org'

gem 'rails', '~> 7.1'
`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "rails")
}

func TestDetect_GemfileRailsInSubdir(t *testing.T) {
	dir := createTempDir(t)
	subdir := dir + "/api"
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTempFile(t, subdir, "Gemfile", `gem 'rails', '~> 7.1'`)
	d := newDetector(t)
	fws := d.Detect(dir)
	expectContains(t, fws, "rails")
}

func TestDetect_NoGemfile(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "package.json", `{"dependencies":{"express":"^4"}}`)
	d := newDetector(t)
	fws := d.Detect(dir)
	if hasFramework(fws, "rails") {
		t.Errorf("expected no 'rails' without Gemfile, got %v", fws)
	}
}

func TestDetect_MalformedGemfile(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "Gemfile", string([]byte{0xff, 0xfe, 0x00}))
	d := newDetector(t)
	fws := d.Detect(dir)
	if hasFramework(fws, "rails") {
		t.Errorf("expected no 'rails' for malformed Gemfile, got %v", fws)
	}
}

func TestDetect_MalformedGoMod(t *testing.T) {
	dir := createTempDir(t)
	writeTempFile(t, dir, "go.mod", string([]byte{0xff, 0xfe, 0x00}))
	d := newDetector(t)
	fws := d.Detect(dir)
	if hasFramework(fws, "go") {
		t.Errorf("expected no 'go' for malformed go.mod, got %v", fws)
	}
}

type awareExtractor struct {
	ext    string
	fws    []string
	called bool
}

func (e *awareExtractor) Supports(ext string) bool {
	return e.ext == ext
}

func (e *awareExtractor) ExtractEdges(filePath string, content []byte) ([]graph.Edge, error) {
	e.called = true
	return nil, nil
}

func (e *awareExtractor) RequiresFrameworks() []string {
	return e.fws
}

type unawareExtractor struct {
	ext    string
	called bool
}

func (e *unawareExtractor) Supports(ext string) bool {
	return e.ext == ext
}

func (e *unawareExtractor) ExtractEdges(filePath string, content []byte) ([]graph.Edge, error) {
	e.called = true
	return nil, nil
}

func TestSetActiveFrameworks_SkipWhenNotInSet(t *testing.T) {
	echoEx := &awareExtractor{ext: ".go", fws: []string{"echo"}}
	ginEx := &awareExtractor{ext: ".go", fws: []string{"gin"}}
	r := graph.NewRegistry(echoEx, ginEx)
	r.SetActiveFrameworks([]string{"echo"})

	_, err := r.ExtractEdges("test.go", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if !echoEx.called {
		t.Error("echo extractor should have been called")
	}
	if ginEx.called {
		t.Error("gin extractor should NOT have been called (gin not in active frameworks)")
	}
}

func TestSetActiveFrameworks_RunWhenInSet(t *testing.T) {
	echoEx := &awareExtractor{ext: ".go", fws: []string{"echo"}}
	r := graph.NewRegistry(echoEx)
	r.SetActiveFrameworks([]string{"echo"})

	_, err := r.ExtractEdges("test.go", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if !echoEx.called {
		t.Error("echo extractor should have been called")
	}
}

func TestSetActiveFrameworks_EmptyFwsAlwaysRuns(t *testing.T) {
	ex := &awareExtractor{ext: ".go", fws: []string{}}
	r := graph.NewRegistry(ex)
	r.SetActiveFrameworks([]string{})
	_, err := r.ExtractEdges("test.go", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if !ex.called {
		t.Error("extractor with empty RequiresFrameworks should always run")
	}
}

func TestSetActiveFrameworks_UnawareExtractorAlwaysRuns(t *testing.T) {
	unEx := &unawareExtractor{ext: ".go"}
	r := graph.NewRegistry(unEx)
	r.SetActiveFrameworks([]string{"gin"})
	_, err := r.ExtractEdges("test.go", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if !unEx.called {
		t.Error("unaware extractor should always run")
	}
}

func expectContains(t *testing.T, fws []string, expected ...string) {
	t.Helper()
	for _, exp := range expected {
		if !hasFramework(fws, exp) {
			t.Errorf("expected %q in %v", exp, fws)
		}
	}
}

func hasFramework(fws []string, fw string) bool {
	for _, f := range fws {
		if f == fw {
			return true
		}
	}
	return false
}
