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
	rbE, _ := symbol.NewRubySymbolExtractor()
	vueE, err := symbol.NewVueExtractor()
	if err != nil {
		t.Fatalf("NewVueExtractor: %v", err)
	}

	reg := symbol.NewRegistry(goE, tsE, pyE, jsE, rbE, vueE)

	for _, ext := range []string{".go", ".ts", ".tsx", ".mts", ".cts", ".py", ".js", ".mjs", ".cjs", ".jsx", ".rb", ".vue"} {
		if !reg.Supports(ext) {
			t.Errorf("should support %s", ext)
		}
	}

	syms, err := reg.Extract("main.go", []byte("package main\nfunc main() {}"))
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Error("registry: expected symbols from .go file")
	}
}

func TestJavaScriptExtractor_ExpressAndModuleIdioms(t *testing.T) {
	e, err := symbol.NewJavaScriptExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`
export const createReserved = function (payload) { return payload; };
const wrappedHandler = catchAsync(async (req, res) => res.json({ ok: true }));
exports.legacyExport = (x) => x + 1;

export default {
  async getUserPendingCardVerifications(userId) {
    return userId;
  },
  findById: async function (id) {
    return id;
  },
  formatRow: (row) => row,
};
`)

	for _, file := range []string{"repo.js", "repo.mjs", "repo.cjs", "view.jsx"} {
		syms, err := e.Extract(file, src)
		if err != nil {
			t.Fatalf("Extract(%s): %v", file, err)
		}
		got := make(map[string]symbol.Kind)
		for _, s := range syms {
			got[s.Name] = s.Kind
		}
		want := map[string]symbol.Kind{
			"createReserved":                  symbol.KindFunction,
			"wrappedHandler":                  symbol.KindFunction,
			"legacyExport":                    symbol.KindFunction,
			"getUserPendingCardVerifications": symbol.KindMethod,
			"findById":                        symbol.KindMethod,
			"formatRow":                       symbol.KindMethod,
		}
		for name, kind := range want {
			if got[name] != kind {
				t.Errorf("%s: symbol %s = %q, want %q (all=%v)", file, name, got[name], kind, got)
			}
		}
	}
}

func TestVueExtractor_ScriptSetupAndOptionsAPI(t *testing.T) {
	e, err := symbol.NewVueExtractor()
	if err != nil {
		t.Fatal(err)
	}

	sfc := []byte(`<template>
  <button @click="submitOrder">{{ totalFormatted }}</button>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface OrderItem {
  id: string
  price: number
}

const totalFormatted = computed(() => '0.00')
const addTradeLog = async (id: string) => id

function submitOrder() {
  return addTradeLog('1')
}
</script>
`)

	syms, err := e.Extract("components/CartPanel.vue", sfc)
	if err != nil {
		t.Fatalf("Extract vue: %v", err)
	}
	got := make(map[string]symbol.Symbol)
	for _, s := range syms {
		got[s.Name] = s
	}

	for name, wantKind := range map[string]symbol.Kind{
		"OrderItem":      symbol.KindInterface,
		"totalFormatted": symbol.KindFunction,
		"addTradeLog":    symbol.KindFunction,
		"submitOrder":    symbol.KindFunction,
	} {
		sym, ok := got[name]
		if !ok {
			t.Errorf("missing Vue symbol %q (got=%v)", name, got)
			continue
		}
		if sym.Kind != wantKind {
			t.Errorf("Vue symbol %q kind = %q, want %q", name, sym.Kind, wantKind)
		}
		if sym.Language != "vue" {
			t.Errorf("Vue symbol %q language = %q, want vue", name, sym.Language)
		}
		if sym.Line < 5 {
			t.Errorf("Vue symbol %q line = %d, expected >= 5 (after <template>)", name, sym.Line)
		}
	}
}
