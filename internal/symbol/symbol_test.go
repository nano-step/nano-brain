package symbol_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/symbol"
)

func TestGoExtractor(t *testing.T) {
	e, err := symbol.NewGoExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`package main

func Greet(name string) string { return "hi " + name }

type User struct { Name string }

type Writer interface { Write([]byte) (int, error) }

const MaxRetry = 3

var DefaultTimeout = 30
`)

	syms, err := e.Extract("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]symbol.Kind{
		"Greet":   symbol.KindFunction,
		"User":    symbol.KindStruct,
		"Writer":  symbol.KindInterface,
		"MaxRetry": symbol.KindConst,
		"DefaultTimeout": symbol.KindVar,
	}
	got := make(map[string]symbol.Kind)
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("Go: %s: want %s got %s", name, kind, got[name])
		}
	}
}

func TestGoMethodExtractor(t *testing.T) {
	e, err := symbol.NewGoExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`package main
type Svc struct{}
func (s *Svc) Handle() error { return nil }
`)
	syms, err := e.Extract("svc.go", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range syms {
		if s.Name == "Handle" && s.Kind != symbol.KindMethod {
			t.Errorf("Handle: want method got %s", s.Kind)
		}
	}
}

func TestTypeScriptExtractor(t *testing.T) {
	e, err := symbol.NewTypeScriptExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`
function greet(name: string): string { return name; }

interface User { name: string; }

type ID = string;

class Service {
  handle(): void {}
}

const fetchUser = (id: string) => id;
`)
	syms, err := e.Extract("app.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]symbol.Kind{
		"greet":    symbol.KindFunction,
		"User":     symbol.KindInterface,
		"ID":       symbol.KindType,
		"Service":  symbol.KindType,
		"fetchUser": symbol.KindFunction,
	}
	got := make(map[string]symbol.Kind)
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("TS: %s: want %s got %s", name, kind, got[name])
		}
	}
}

func TestPythonExtractor(t *testing.T) {
	e, err := symbol.NewPythonExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`
def greet(name):
    return name

class User:
    def __init__(self):
        pass
`)
	syms, err := e.Extract("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]symbol.Kind{
		"greet": symbol.KindFunction,
		"User":  symbol.KindType,
		"__init__": symbol.KindMethod,
	}
	got := make(map[string]symbol.Kind)
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("Python: %s: want %s got %s", name, kind, got[name])
		}
	}
}

func TestJavaScriptExtractor(t *testing.T) {
	e, err := symbol.NewJavaScriptExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`
function greet(name) { return name; }

class Service {
  handle() {}
}

const fetchUser = (id) => id;
`)
	syms, err := e.Extract("app.js", src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]symbol.Kind{
		"greet":    symbol.KindFunction,
		"Service":  symbol.KindType,
		"fetchUser": symbol.KindFunction,
	}
	got := make(map[string]symbol.Kind)
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("JS: %s: want %s got %s", name, kind, got[name])
		}
	}
}

func TestRegistry(t *testing.T) {
	goE, _ := symbol.NewGoExtractor()
	tsE, _ := symbol.NewTypeScriptExtractor()
	pyE, _ := symbol.NewPythonExtractor()
	jsE, _ := symbol.NewJavaScriptExtractor()

	reg := symbol.NewRegistry(goE, tsE, pyE, jsE)

	if !reg.Supports(".go") { t.Error("should support .go") }
	if !reg.Supports(".ts") { t.Error("should support .ts") }
	if !reg.Supports(".tsx") { t.Error("should support .tsx") }
	if !reg.Supports(".py") { t.Error("should support .py") }
	if !reg.Supports(".js") { t.Error("should support .js") }
	if reg.Supports(".rb") { t.Error("should not support .rb") }

	syms, err := reg.Extract("main.go", []byte("package main\nfunc main() {}"))
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Error("registry: expected symbols from .go file")
	}
}
