package chunker_test

import (
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/chunker"
	"github.com/rs/zerolog"
)

func TestSymbolAwareChunker_GoFunctions(t *testing.T) {
	src := `package main

func Foo() {
	println("foo")
}

func Bar() {
	println("bar")
}

func Baz() {
	println("baz")
}

func Qux() {
	println("qux")
}

func Quux() {
	println("quux")
}
`
	logger := zerolog.Nop()
	fallback := chunker.NewFixedChunker()
	sc, err := chunker.NewSymbolAwareChunker(fallback, logger)
	if err != nil {
		t.Fatal(err)
	}

	chunks := sc.Chunk(src, "main.go")
	if len(chunks) != 5 {
		t.Fatalf("expected 5 chunks, got %d", len(chunks))
	}

	for _, c := range chunks {
		if c.ChunkType != chunker.ChunkTypeSymbol {
			t.Errorf("expected chunk_type=symbol, got %s", c.ChunkType)
		}
		if c.Language != "go" {
			t.Errorf("expected language=go, got %s", c.Language)
		}
		if c.SymbolName == "" {
			t.Error("expected non-empty symbol_name")
		}
		if c.SymbolKind != "function" {
			t.Errorf("expected kind=function, got %s", c.SymbolKind)
		}
	}
}

func TestSymbolAwareChunker_LargeFunction_Fallback(t *testing.T) {
	body := strings.Repeat("\tprintln(\"x\")\n", 600)
	src := "package main\n\nfunc Big() {\n" + body + "}\n"

	logger := zerolog.Nop()
	fallback := chunker.NewFixedChunker()
	sc, err := chunker.NewSymbolAwareChunker(fallback, logger)
	if err != nil {
		t.Fatal(err)
	}

	chunks := sc.Chunk(src, "big.go")
	if len(chunks) < 2 {
		t.Fatalf("expected >1 chunks for large function, got %d", len(chunks))
	}

	for _, c := range chunks {
		if c.SymbolName != "Big" {
			t.Errorf("expected symbol_name=Big, got %s", c.SymbolName)
		}
	}
}

func TestSymbolAwareChunker_ParseFailure_Fallback(t *testing.T) {
	src := "this is not valid go code {{{{{{{"

	logger := zerolog.Nop()
	fallback := chunker.NewFixedChunker()
	sc, err := chunker.NewSymbolAwareChunker(fallback, logger)
	if err != nil {
		t.Fatal(err)
	}

	chunks := sc.Chunk(src, "bad.go")
	if len(chunks) == 0 {
		t.Fatal("expected fallback to produce chunks")
	}
	for _, c := range chunks {
		if c.ChunkType != chunker.ChunkTypeRaw {
			t.Errorf("expected chunk_type=raw from fallback, got %s", c.ChunkType)
		}
	}
}

func TestSymbolAwareChunker_EmptyFile_Fallback(t *testing.T) {
	logger := zerolog.Nop()
	fallback := chunker.NewFixedChunker()
	sc, err := chunker.NewSymbolAwareChunker(fallback, logger)
	if err != nil {
		t.Fatal(err)
	}

	chunks := sc.Chunk("", "empty.go")
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty file, got %d", len(chunks))
	}
}
