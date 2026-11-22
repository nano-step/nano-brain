package symbol_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/symbol"
)

func buildBodyLines704(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  const step%d = %d;\n", i, i)
	}
	return b.String()
}

func findSymbol704(syms []symbol.Symbol, name string, kind symbol.Kind) *symbol.Symbol {
	for i := range syms {
		if syms[i].Name == name && syms[i].Kind == kind {
			return &syms[i]
		}
	}
	return nil
}

func TestDedupe_TopLevelWinsOverNestedAndAssignment_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)
	body50 := buildBodyLines704(48) + "  return step0;\n"

	tsNestedThenTop := "function setupHelper() {\n  const foo = () => 1;\n  return foo();\n}\n\nexport const foo = async (req: Req): Promise<number> => {\n" + body50 + "};\n"
	jsNestedThenTop := "function setupHelper() {\n  const foo = () => 1;\n  return foo();\n}\n\nexport const foo = async (req) => {\n" + body50 + "};\n"
	vueNestedThenTop := "<template>\n  <div>{{ ready }}</div>\n</template>\n\n<script setup lang=\"ts\">\nfunction setupHelper() {\n  const foo = () => 1;\n  return foo();\n}\n\nexport const foo = async (req: Req): Promise<number> => {\n" + body50 + "};\n</script>\n"
	vueMultiScript := "<script lang=\"ts\">\nfunction helperInNormalScript() {\n  const foo = () => 1;\n  return foo();\n}\n</script>\n\n<script setup lang=\"ts\">\nexport const foo = async (req: Req): Promise<number> => {\n" + body50 + "};\n</script>\n"

	tsAssignThenTop := "class Holder {\n  init() {\n    this.foo = () => 1;\n  }\n}\nexports.foo = () => 2;\nmodule.exports.standaloneBar = (x: number) => {\n  return x + 1;\n};\n\nexport function foo(req: Req): number {\n" + body50 + "}\n"
	tsTopThenNestedAndAssign := "export const foo = async (req: Req): Promise<number> => {\n" + body50 + "};\n\nexports.foo = () => 2;\nfunction laterHelper() {\n  const foo = () => 1;\n  return foo();\n}\n"
	jsAssignThenTop := "class Holder {\n  init() {\n    this.foo = () => 1;\n  }\n}\nexports.foo = () => 2;\nmodule.exports.standaloneBar = (x) => {\n  return x + 1;\n};\n\nexport const foo = (req) => {\n" + body50 + "};\n"

	cases := []struct {
		name        string
		file        string
		src         string
		wantLine    int
		wantEndLine int
		wantBar     bool
	}{
		{name: "ts_nested_before_top_level", file: "src/service.ts", src: tsNestedThenTop, wantLine: 6, wantEndLine: 56},
		{name: "tsx_nested_before_top_level", file: "src/service.tsx", src: tsNestedThenTop, wantLine: 6, wantEndLine: 56},
		{name: "js_nested_before_top_level", file: "src/service.js", src: jsNestedThenTop, wantLine: 6, wantEndLine: 56},
		{name: "vue_nested_before_top_level", file: "src/Service.vue", src: vueNestedThenTop, wantLine: 11, wantEndLine: 61},
		{name: "vue_multi_script_nested_before_top_level", file: "src/Multi.vue", src: vueMultiScript, wantLine: 9, wantEndLine: 59},
		{name: "ts_member_assign_before_top_level", file: "src/assign.ts", src: tsAssignThenTop, wantLine: 11, wantEndLine: 61, wantBar: true},
		{name: "ts_top_level_before_nested_and_assign", file: "src/top_first.ts", src: tsTopThenNestedAndAssign, wantLine: 1, wantEndLine: 51},
		{name: "js_member_assign_before_top_level", file: "src/assign.js", src: jsAssignThenTop, wantLine: 11, wantEndLine: 61, wantBar: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			syms, err := reg.Extract(tc.file, []byte(tc.src))
			if err != nil {
				t.Fatalf("Extract(%s): %v", tc.file, err)
			}
			got := findSymbol704(syms, "foo", symbol.KindFunction)
			if got == nil {
				t.Fatalf("%s: missing function foo in %+v", tc.file, syms)
			}
			if got.Line != tc.wantLine || got.EndLine != tc.wantEndLine {
				t.Errorf("%s: foo span = %d..%d, want %d..%d (sig=%q)", tc.file, got.Line, got.EndLine, tc.wantLine, tc.wantEndLine, got.Signature)
			}
			if tc.wantBar {
				bar := findSymbol704(syms, "standaloneBar", symbol.KindFunction)
				if bar == nil {
					t.Errorf("%s: standalone module.exports.standaloneBar was not extracted; got %+v", tc.file, syms)
				} else if bar.Line != 7 || bar.EndLine != 9 {
					t.Errorf("%s: standaloneBar span = %d..%d, want 7..9", tc.file, bar.Line, bar.EndLine)
				}
			}
		})
	}
}

func TestSatisfiesAsAndParenthesizedFunctions_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	tsSrc := `export const foo = (async (req: Req) => {
  const ok = await check(req);
  return ok;
}) satisfies Handler;

export const bar = ((req: Req): Res => {
  return build(req);
}) as Handler;

export const baz = async () => {
  await flush();
} satisfies () => Promise<void>;

const qux = ((x: number) => x + 1);

const notAFunc = ({ port: 3000 }) satisfies ServerConfig;
const notAFuncAs = (42) as number;
`

	vueSrc := "<template>\n  <div>{{ ready }}</div>\n</template>\n\n<script setup lang=\"ts\">\n" + tsSrc + "</script>\n"

	for _, tc := range []struct {
		file   string
		src    string
		offset int
	}{
		{file: "src/handlers.ts", src: tsSrc, offset: 0},
		{file: "src/handlers.tsx", src: tsSrc, offset: 0},
		{file: "src/Handlers.vue", src: vueSrc, offset: 5},
	} {
		t.Run(tc.file, func(t *testing.T) {
			syms, err := reg.Extract(tc.file, []byte(tc.src))
			if err != nil {
				t.Fatalf("Extract(%s): %v", tc.file, err)
			}
			for _, want := range []struct {
				name    string
				line    int
				endLine int
			}{
				{name: "foo", line: 1 + tc.offset, endLine: 4 + tc.offset},
				{name: "bar", line: 6 + tc.offset, endLine: 8 + tc.offset},
				{name: "baz", line: 10 + tc.offset, endLine: 12 + tc.offset},
				{name: "qux", line: 14 + tc.offset, endLine: 14 + tc.offset},
			} {
				got := findSymbol704(syms, want.name, symbol.KindFunction)
				if got == nil {
					t.Errorf("%s: missing function symbol %q in %+v", tc.file, want.name, syms)
					continue
				}
				if got.Line != want.line || got.EndLine != want.endLine {
					t.Errorf("%s: %s span = %d..%d, want %d..%d", tc.file, want.name, got.Line, got.EndLine, want.line, want.endLine)
				}
			}
			for _, forbidden := range []string{"notAFunc", "notAFuncAs"} {
				if got := findSymbol704(syms, forbidden, symbol.KindFunction); got != nil {
					t.Errorf("%s: non-function %q must not be extracted, got %+v", tc.file, forbidden, *got)
				}
			}
		})
	}

	jsSrc := `const qux = ((x) => {
  return x + 1;
});
`
	jsSyms, err := reg.Extract("src/parens.js", []byte(jsSrc))
	if err != nil {
		t.Fatalf("Extract(src/parens.js): %v", err)
	}
	if got := findSymbol704(jsSyms, "qux", symbol.KindFunction); got == nil || got.Line != 1 || got.EndLine != 3 {
		t.Errorf("src/parens.js: want qux 1..3, got %+v (all=%+v)", got, jsSyms)
	}

	tsAngleAndChained := `const angled = <Handler>((req: Req) => {
  return req;
});
export const chained = ((req: Req) => {
  return req;
}) as unknown as Handler;
`
	angleSyms, err := reg.Extract("src/assertions.ts", []byte(tsAngleAndChained))
	if err != nil {
		t.Fatalf("Extract(src/assertions.ts): %v", err)
	}
	if got := findSymbol704(angleSyms, "angled", symbol.KindFunction); got == nil || got.Line != 1 || got.EndLine != 3 {
		t.Errorf("src/assertions.ts: angled got %+v, want 1..3", got)
	}
	if got := findSymbol704(angleSyms, "chained", symbol.KindFunction); got == nil || got.Line != 4 || got.EndLine != 6 {
		t.Errorf("src/assertions.ts: chained got %+v, want 4..6", got)
	}
}

func TestHOFWrappedFunctionsAndNoCollectionCallbackFalsePositives_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	tsxSrc := `export const MemoCard = memo((props: Props) => <div/>);

export const FwdInput = memo(forwardRef((props: InputProps, ref: Ref<HTMLInputElement>) => {
  return <input ref={ref}/>;
}));

export const ReactFwd = React.memo(React.forwardRef((props: InputProps, ref: Ref<HTMLInputElement>) => <input ref={ref}/>));

export const GenericMemo = memo<Props>((props: Props) => <div/>);

var legacyWrapped = debounce(function(x: number) {
  return x;
});

const ids = items.map((x: Item) => x.id);
const total = nums.reduce((a: number, b: number) => a + b, 0);
`
	tsxSyms, err := reg.Extract("src/Components.tsx", []byte(tsxSrc))
	if err != nil {
		t.Fatalf("Extract(src/Components.tsx): %v", err)
	}
	for _, want := range []struct {
		name    string
		line    int
		endLine int
	}{
		{name: "MemoCard", line: 1, endLine: 1},
		{name: "FwdInput", line: 3, endLine: 5},
		{name: "ReactFwd", line: 7, endLine: 7},
		{name: "GenericMemo", line: 9, endLine: 9},
		{name: "legacyWrapped", line: 11, endLine: 13},
	} {
		got := findSymbol704(tsxSyms, want.name, symbol.KindFunction)
		if got == nil {
			t.Errorf("Components.tsx: missing symbol %q in %+v", want.name, tsxSyms)
			continue
		}
		if got.Line != want.line || got.EndLine != want.endLine {
			t.Errorf("Components.tsx: %s span = %d..%d, want %d..%d", want.name, got.Line, got.EndLine, want.line, want.endLine)
		}
	}
	for _, forbidden := range []string{"ids", "total"} {
		if got := findSymbol704(tsxSyms, forbidden, symbol.KindFunction); got != nil {
			t.Errorf("Components.tsx: eager collection variable %q must not be extracted as function, got %+v", forbidden, *got)
		}
	}

	tsSrc := `export const wrapped = withAuth(catchAsync(async (req: Req) => {
  return req;
}));
var legacyWrapped = debounce(function(x: number) {
  return x;
});
const ids = items.map((x: Item) => x.id);
const total = nums.reduce((a: number, b: number) => a + b, 0);
`
	for _, tc := range []struct {
		file   string
		src    string
		offset int
	}{
		{file: "src/hof.ts", src: tsSrc, offset: 0},
		{file: "src/Hof.vue", src: "<script setup lang=\"ts\">\n" + tsSrc + "</script>\n", offset: 1},
	} {
		t.Run(tc.file, func(t *testing.T) {
			syms, err := reg.Extract(tc.file, []byte(tc.src))
			if err != nil {
				t.Fatalf("Extract(%s): %v", tc.file, err)
			}
			for _, want := range []struct {
				name    string
				line    int
				endLine int
			}{
				{name: "wrapped", line: 1 + tc.offset, endLine: 3 + tc.offset},
				{name: "legacyWrapped", line: 4 + tc.offset, endLine: 6 + tc.offset},
			} {
				got := findSymbol704(syms, want.name, symbol.KindFunction)
				if got == nil || got.Line != want.line || got.EndLine != want.endLine {
					t.Errorf("%s: %s got %+v, want %d..%d (all=%+v)", tc.file, want.name, got, want.line, want.endLine, syms)
				}
			}
			for _, forbidden := range []string{"ids", "total"} {
				if got := findSymbol704(syms, forbidden, symbol.KindFunction); got != nil {
					t.Errorf("%s: eager collection variable %q must not be extracted as function, got %+v", tc.file, forbidden, *got)
				}
			}
		})
	}

	jsSrc := `export const FwdInput = memo(forwardRef((props, ref) => {
  return props;
}));
var legacyWrapped = debounce(function(x) {
  return x;
});
const ids = items.map((x) => x.id);
const total = nums.reduce((a, b) => a + b, 0);
`
	jsSyms, err := reg.Extract("src/hof.js", []byte(jsSrc))
	if err != nil {
		t.Fatalf("Extract(src/hof.js): %v", err)
	}
	if got := findSymbol704(jsSyms, "FwdInput", symbol.KindFunction); got == nil || got.Line != 1 || got.EndLine != 3 {
		t.Errorf("hof.js: FwdInput got %+v, want 1..3 (all=%+v)", got, jsSyms)
	}
	if got := findSymbol704(jsSyms, "legacyWrapped", symbol.KindFunction); got == nil || got.Line != 4 || got.EndLine != 6 {
		t.Errorf("hof.js: legacyWrapped got %+v, want 4..6 (all=%+v)", got, jsSyms)
	}
	for _, forbidden := range []string{"ids", "total"} {
		if got := findSymbol704(jsSyms, forbidden, symbol.KindFunction); got != nil {
			t.Errorf("hof.js: eager collection variable %q must not be extracted as function, got %+v", forbidden, *got)
		}
	}
}

func TestClassFieldArrows_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	tsClass := `export class Controller {
  handle = async (req: Req): Promise<Res> => {
    const out = await service(req);
    return out;
  };
  private onClick = (e: Event): void => {
    e.preventDefault();
  };
  legacyFn = function(x: number): number {
    return x + 1;
  };
  typedField = ((e: Event) => {
    return e;
  }) satisfies EventListener;
  scalar = 42;
}
`
	jsClass := `export class Controller {
  handle = async (req) => {
    const out = await service(req);
    return out;
  };
  onClick = (e) => {
    e.preventDefault();
  };
  legacyFn = function(x) {
    return x + 1;
  };
  scalar = 42;
}
`

	for _, tc := range []struct {
		file   string
		src    string
		offset int
	}{
		{file: "src/controller.ts", src: tsClass, offset: 0},
		{file: "src/controller.tsx", src: tsClass, offset: 0},
		{file: "src/controller.js", src: jsClass, offset: 0},
		{file: "src/controller.jsx", src: jsClass, offset: 0},
		{file: "src/Controller.vue", src: "<script setup lang=\"ts\">\n" + tsClass + "</script>\n", offset: 1},
	} {
		t.Run(tc.file, func(t *testing.T) {
			syms, err := reg.Extract(tc.file, []byte(tc.src))
			if err != nil {
				t.Fatalf("Extract(%s): %v", tc.file, err)
			}
			for _, want := range []struct {
				name    string
				line    int
				endLine int
			}{
				{name: "handle", line: 2 + tc.offset, endLine: 5 + tc.offset},
				{name: "onClick", line: 6 + tc.offset, endLine: 8 + tc.offset},
				{name: "legacyFn", line: 9 + tc.offset, endLine: 11 + tc.offset},
			} {
				got := findSymbol704(syms, want.name, symbol.KindMethod)
				if got == nil {
					t.Errorf("%s: missing class-field method %q in %+v", tc.file, want.name, syms)
					continue
				}
				if got.Line != want.line || got.EndLine != want.endLine {
					t.Errorf("%s: %s span = %d..%d, want %d..%d", tc.file, want.name, got.Line, got.EndLine, want.line, want.endLine)
				}
			}
			if strings.HasSuffix(tc.file, ".ts") || strings.HasSuffix(tc.file, ".tsx") || strings.HasSuffix(tc.file, ".vue") {
				got := findSymbol704(syms, "typedField", symbol.KindMethod)
				if got == nil || got.Line != 12+tc.offset || got.EndLine != 14+tc.offset {
					t.Errorf("%s: typedField got %+v, want %d..%d", tc.file, got, 12+tc.offset, 14+tc.offset)
				}
			}
			for _, s := range syms {
				if s.Name == "scalar" {
					t.Errorf("%s: non-function class field scalar must not be extracted, got %+v", tc.file, s)
				}
			}
		})
	}
}
