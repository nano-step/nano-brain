package symbol_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/symbol"
)

func newProdSymbolRegistry(t *testing.T) *symbol.Registry {
	t.Helper()
	goE, err := symbol.NewGoExtractor()
	if err != nil {
		t.Fatal(err)
	}
	tsE, err := symbol.NewTypeScriptExtractor()
	if err != nil {
		t.Fatal(err)
	}
	pyE, err := symbol.NewPythonExtractor()
	if err != nil {
		t.Fatal(err)
	}
	jsE, err := symbol.NewJavaScriptExtractor()
	if err != nil {
		t.Fatal(err)
	}
	rbE, err := symbol.NewRubySymbolExtractor()
	if err != nil {
		t.Fatal(err)
	}
	vueE, err := symbol.NewVueExtractor()
	if err != nil {
		t.Fatal(err)
	}
	return symbol.NewRegistry(goE, tsE, pyE, jsE, rbE, vueE)
}

func TestRegistry_TypedFirstParamArrows_703(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	cases := []struct {
		name        string
		file        string
		src         string
		wantName    string
		wantLine    int
		wantEndLine int
	}{
		{
			name:        "ts_single_typed_param",
			file:        "src/a.ts",
			src:         "const Foo = (p: P) => 1;\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "tsx_single_typed_param",
			file:        "src/a.tsx",
			src:         "const Foo = (p: P) => 1;\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "ts_typed_param_with_return_type",
			file:        "src/b.ts",
			src:         "const Foo = (p: string): number => Number(p);\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "tsx_typed_param_with_return_type",
			file:        "src/b.tsx",
			src:         "const Foo = (p: string): number => Number(p);\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "ts_multi_typed_params",
			file:        "src/c.ts",
			src:         "const Foo = (p: P, q: Q) => p;\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "tsx_multi_typed_params",
			file:        "src/c.tsx",
			src:         "const Foo = (p: P, q: Q) => p;\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "ts_destructured_typed_param",
			file:        "src/d.ts",
			src:         "const Foo = ({ a }: P) => a;\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "tsx_destructured_typed_param",
			file:        "src/d.tsx",
			src:         "const Foo = ({ a }: P) => a;\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "ts_exported_multiline_typed_arrow",
			file:        "src/e.ts",
			src:         "export const Foo = (a: string, b: number): void => {\n  consume(a);\n  consume(b);\n}\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 4,
		},
		{
			name:        "tsx_exported_multiline_typed_arrow",
			file:        "src/e.tsx",
			src:         "export const Foo = (a: string, b: number): void => {\n  consume(a);\n  consume(b);\n}\n",
			wantName:    "Foo",
			wantLine:    1,
			wantEndLine: 4,
		},
		{
			name:        "tsx_jsx_component_typed_props",
			file:        "src/Card.tsx",
			src:         "export const Card = (props: Props) => <div/>;\n",
			wantName:    "Card",
			wantLine:    1,
			wantEndLine: 1,
		},
		{
			name:        "tsx_outer_async_with_inner_typed_map_callback",
			file:        "src/loader.tsx",
			src:         "export const load = async (id: string): Promise<void> => {\n  const xs = items.map((x: Item) => x.id);\n  await persist(id, xs);\n}\n",
			wantName:    "load",
			wantLine:    1,
			wantEndLine: 4,
		},
		{
			name:        "vue_script_setup_ts_outer_async_with_inner_typed_map_callback",
			file:        "src/Loader.vue",
			src:         "<template>\n  <div>{{ ready }}</div>\n</template>\n\n<script setup lang=\"ts\">\nexport const load = async (id: string): Promise<void> => {\n  const xs = items.map((x: Item) => x.id);\n  await persist(id, xs);\n}\n</script>\n",
			wantName:    "load",
			wantLine:    6,
			wantEndLine: 9,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			syms, err := reg.Extract(tc.file, []byte(tc.src))
			if err != nil {
				t.Fatalf("Extract(%s): %v", tc.file, err)
			}
			var found *symbol.Symbol
			for i := range syms {
				if syms[i].Name == tc.wantName {
					found = &syms[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("%s: missing symbol %q; got %+v", tc.file, tc.wantName, syms)
			}
			if found.Kind != symbol.KindFunction {
				t.Errorf("%s: %s kind = %q, want %q", tc.file, tc.wantName, found.Kind, symbol.KindFunction)
			}
			if found.Line != tc.wantLine || found.EndLine != tc.wantEndLine {
				t.Errorf("%s: %s span = %d..%d, want %d..%d", tc.file, tc.wantName, found.Line, found.EndLine, tc.wantLine, tc.wantEndLine)
			}
		})
	}
}

func TestRegistry_LargeJavaScriptFileNoEarlyStopDrop_703(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	var b strings.Builder
	// 500 10-line functions = 5000 lines; fn495 starts past line 4950.
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "export const fn%03d = (req, res, next) => {\n", i)
		b.WriteString("  const payload = { a: req.body?.a ?? 1, b: req.query?.b ?? 2, c: [1, 2, 3].map((v) => ({ v, ok: v > 0 })) };\n")
		b.WriteString("  if (!payload.a) {\n")
		b.WriteString("    return res.status(400).json({ error: 'bad_request', payload });\n")
		b.WriteString("  }\n")
		b.WriteString("  const out = Object.entries(payload).reduce((acc, [k, v]) => ({ ...acc, [k]: v }), {});\n")
		b.WriteString("  next(null, out);\n")
		b.WriteString("  return out;\n")
		b.WriteString("};\n\n")
	}

	src := []byte(b.String())
	totalLines := strings.Count(b.String(), "\n")
	if totalLines < 5000 {
		t.Fatalf("fixture too short: %d lines", totalLines)
	}

	syms, err := reg.Extract("src/large_service.js", src)
	if err != nil {
		t.Fatalf("Extract(src/large_service.js): %v", err)
	}

	var tail *symbol.Symbol
	for i := range syms {
		if syms[i].Name == "fn499" {
			tail = &syms[i]
			break
		}
	}
	if tail == nil {
		t.Fatalf("expected tail symbol fn499 past line 4900 to be extracted (got %d symbols total)", len(syms))
	}
	if tail.Line <= 4900 {
		t.Fatalf("expected fn499 line > 4900, got %d", tail.Line)
	}
}
