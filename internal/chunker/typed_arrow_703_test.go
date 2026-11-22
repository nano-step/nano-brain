package chunker_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/chunker"
	"github.com/rs/zerolog"
)

func TestSymbolAwareChunker_TypedFirstParamArrow_703(t *testing.T) {
	sc, err := chunker.NewSymbolAwareChunker(chunker.NewFixedChunker(), zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		file string
		src  string
	}{
		{
			name: "ts",
			file: "src/foo.ts",
			src:  "export const Foo = (p: Props): Result => {\n  return render(p);\n}\n",
		},
		{
			name: "tsx",
			file: "src/Foo.tsx",
			src:  "export const Foo = (p: Props): JSX.Element => {\n  return <div/>;\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := sc.Chunk(tc.src, tc.file)
			if len(chunks) != 1 {
				t.Fatalf("expected 1 chunk, got %d: %+v", len(chunks), chunks)
			}
			got := chunks[0]
			if got.ChunkType != chunker.ChunkTypeSymbol || got.SymbolName != "Foo" || got.StartLine != 1 || got.EndLine != 3 {
				t.Fatalf("got chunk type=%q symbol=%q span=%d..%d, want symbol Foo 1..3", got.ChunkType, got.SymbolName, got.StartLine, got.EndLine)
			}
		})
	}
}
