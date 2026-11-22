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

func TestCurriedAndBuilderHOFsAndScopedBlocking_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	tsxSrc := `export const ConnectedList = connect(mapState)((props: Props) => <div/>);
export const StyledButton = styled('button')((props: BtnProps) => <button/>);
export const useCart = create<CartState>()((set) => ({ count: 0 }));
export const validatedSave = validate(schema)(async (req: Req) => {
  return req;
});
export const getOrders = protectedProcedure.input(schema).query(async ({ input }) => {
  return input;
});
export const healthQuery = t.procedure.query(() => 'ok');
export const serverLoad = createServerFn({ method: 'GET' }).handler(async () => {
  return { ok: true };
});
export const useCartStore = defineStore('cart', () => {
  return { items: [] };
});
export const doubled = computed(() => count.value * 2);

const winTimer = window.setTimeout(() => {}, 100);
const globalInterval = globalThis.setInterval(() => {}, 1000);
const unsub = bus.on('evt', () => {});
const grouped = Object.groupBy(items, (x: Item) => x.kind);
const suite = describe('suite', () => {});
const arrNoise = [1, 2].custom(() => {});
const cfg = {
  listener: bus.on('x', () => {}),
  directHelper: (x: number) => x + 1,
};
class NoiseHolder {
  timer = setTimeout(() => {}, 100);
  wrappedField = memo(() => {});
  run() {
    this.memberUnsub = bus.on('evt', () => {});
  }
}
`
	syms, err := reg.Extract("src/curried_hof.tsx", []byte(tsxSrc))
	if err != nil {
		t.Fatalf("Extract(src/curried_hof.tsx): %v", err)
	}

	for _, want := range []struct {
		name    string
		line    int
		endLine int
	}{
		{name: "ConnectedList", line: 1, endLine: 1},
		{name: "StyledButton", line: 2, endLine: 2},
		{name: "useCart", line: 3, endLine: 3},
		{name: "validatedSave", line: 4, endLine: 6},
		{name: "getOrders", line: 7, endLine: 9},
		{name: "healthQuery", line: 10, endLine: 10},
		{name: "serverLoad", line: 11, endLine: 13},
		{name: "useCartStore", line: 14, endLine: 16},
		{name: "doubled", line: 17, endLine: 17},
	} {
		got := findSymbol704(syms, want.name, symbol.KindFunction)
		if got == nil {
			t.Errorf("missing HOF symbol %q in %+v", want.name, syms)
			continue
		}
		if got.Line != want.line || got.EndLine != want.endLine {
			t.Errorf("%s span = %d..%d, want %d..%d", want.name, got.Line, got.EndLine, want.line, want.endLine)
		}
	}

	if got := findSymbol704(syms, "directHelper", symbol.KindMethod); got == nil {
		t.Errorf("expected direct object-literal arrow directHelper as KindMethod, got %+v", syms)
	}

	for _, forbidden := range []string{
		"winTimer", "globalInterval", "unsub", "grouped", "suite", "arrNoise",
		"listener", "timer", "wrappedField", "memberUnsub",
	} {
		for _, s := range syms {
			if s.Name == forbidden {
				t.Errorf("noise symbol %q must not be extracted, got %+v", forbidden, s)
			}
		}
	}

	vueHofSrc := `<template><div>{{ doubled }}</div></template>
<script setup lang="ts">
const useCartStore = defineStore('cart', () => {
  return { items: [] };
});
const doubled = computed(() => count.value * 2);
const unsub = bus.on('evt', () => {});
</script>
`
	vueSyms, err := reg.Extract("src/Cart.vue", []byte(vueHofSrc))
	if err != nil {
		t.Fatalf("Extract(src/Cart.vue): %v", err)
	}
	if got := findSymbol704(vueSyms, "useCartStore", symbol.KindFunction); got == nil || got.Line != 3 || got.EndLine != 5 {
		t.Errorf("Cart.vue: useCartStore got %+v, want 3..5 (all=%+v)", got, vueSyms)
	}
	if got := findSymbol704(vueSyms, "doubled", symbol.KindFunction); got == nil || got.Line != 6 || got.EndLine != 6 {
		t.Errorf("Cart.vue: doubled got %+v, want 6..6 (all=%+v)", got, vueSyms)
	}
	if got := findSymbol704(vueSyms, "unsub", symbol.KindFunction); got != nil {
		t.Errorf("Cart.vue: unsub must not be extracted, got %+v", *got)
	}
}

func TestUnwrapJSExprSkipsComments_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	tsxOnly := `export const pureSatisfies = (
  /* leading comment */
  async (req: Req) => {
    return req;
  }
) satisfies Handler;

export const PureMemo = (/*#__PURE__*/ memo((props: Props) => <div/>));

export const PureCalleeMemo = (/*#__PURE__*/ memo)((props: Props) => <div/>);
`
	tsxSyms, err := reg.Extract("src/comments.tsx", []byte(tsxOnly))
	if err != nil {
		t.Fatalf("Extract(src/comments.tsx): %v", err)
	}
	for _, want := range []struct {
		name    string
		line    int
		endLine int
	}{
		{name: "pureSatisfies", line: 1, endLine: 6},
		{name: "PureMemo", line: 8, endLine: 8},
		{name: "PureCalleeMemo", line: 10, endLine: 10},
	} {
		got := findSymbol704(tsxSyms, want.name, symbol.KindFunction)
		if got == nil || got.Line != want.line || got.EndLine != want.endLine {
			t.Errorf("comments.tsx: %s got %+v, want %d..%d (all=%+v)", want.name, got, want.line, want.endLine, tsxSyms)
		}
	}

	tsAngleSrc := `export const pureSatisfies = (
  /* leading comment */
  async (req: Req) => {
    return req;
  }
) satisfies Handler;

export const angledWithComment = <Handler>( // trailing line comment
  (req: Req) => {
    return req;
  }
);
`
	syms, err := reg.Extract("src/angle_comments.ts", []byte(tsAngleSrc))
	if err != nil {
		t.Fatalf("Extract(src/angle_comments.ts): %v", err)
	}
	for _, want := range []struct {
		name    string
		line    int
		endLine int
	}{
		{name: "pureSatisfies", line: 1, endLine: 6},
		{name: "angledWithComment", line: 8, endLine: 12},
	} {
		got := findSymbol704(syms, want.name, symbol.KindFunction)
		if got == nil || got.Line != want.line || got.EndLine != want.endLine {
			t.Errorf("angle_comments.ts: %s got %+v, want %d..%d (all=%+v)", want.name, got, want.line, want.endLine, syms)
		}
	}
}

func TestSignatureDecoratedFieldAndMultiDeclarator_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	tsSrc := `export class FormController {
  @bound
  onSubmit = async (e: Event) => {
    e.preventDefault();
  };
}

const TIMEOUT = 5000,
  fetchWithRetry = async (url: string) => {
    const res = await fetch(url);
    return res;
  };
`
	syms, err := reg.Extract("src/form.ts", []byte(tsSrc))
	if err != nil {
		t.Fatalf("Extract(src/form.ts): %v", err)
	}

	onSubmit := findSymbol704(syms, "onSubmit", symbol.KindMethod)
	if onSubmit == nil {
		t.Fatalf("missing method onSubmit in %+v", syms)
	}
	if onSubmit.Line != 3 || onSubmit.EndLine != 5 {
		t.Errorf("onSubmit span = %d..%d, want 3..5", onSubmit.Line, onSubmit.EndLine)
	}
	if !strings.Contains(onSubmit.Signature, "onSubmit") || onSubmit.Signature == "@bound" {
		t.Errorf("onSubmit Signature = %q, want signature containing onSubmit (not @bound)", onSubmit.Signature)
	}

	fetchSym := findSymbol704(syms, "fetchWithRetry", symbol.KindFunction)
	if fetchSym == nil {
		t.Fatalf("missing function fetchWithRetry in %+v", syms)
	}
	if fetchSym.Line != 9 || fetchSym.EndLine != 12 {
		t.Errorf("fetchWithRetry span = %d..%d, want 9..12", fetchSym.Line, fetchSym.EndLine)
	}
	if !strings.Contains(fetchSym.Signature, "fetchWithRetry") || strings.Contains(fetchSym.Signature, "TIMEOUT") {
		t.Errorf("fetchWithRetry Signature = %q, want signature containing fetchWithRetry without TIMEOUT", fetchSym.Signature)
	}
}

func TestRankingClassExprNamespaceAndExportEdge_704(t *testing.T) {
	reg := newProdSymbolRegistry(t)

	classExprSrc := `const fallback = { handle: () => 0 };

export const RealController = class {
  handle = async (req: Req) => {
    const res = await process(req);
    return res;
  };
};
`
	syms, err := reg.Extract("src/class_expr.ts", []byte(classExprSrc))
	if err != nil {
		t.Fatalf("Extract(src/class_expr.ts): %v", err)
	}
	if got := findSymbol704(syms, "handle", symbol.KindMethod); got == nil || got.Line != 4 || got.EndLine != 7 {
		t.Errorf("class_expr.ts: handle got %+v, want 4..7 (all=%+v)", got, syms)
	}

	nsSrc := `function unrelatedHelper() {
  const foo = () => 1;
  return foo();
}

export namespace Orders {
  export function foo(orderId: string) {
    const normalized = orderId.trim();
    return normalized;
  }
}
`
	nsSyms, err := reg.Extract("src/namespace.ts", []byte(nsSrc))
	if err != nil {
		t.Fatalf("Extract(src/namespace.ts): %v", err)
	}
	if got := findSymbol704(nsSyms, "foo", symbol.KindFunction); got == nil || got.Line != 7 || got.EndLine != 10 {
		t.Errorf("namespace.ts: foo got %+v, want 7..10 (all=%+v)", got, nsSyms)
	}

	exportEdgeSrc := `class noopAdapter {
  handle() {}
}

export class OrderController {
  handle(req: Req) {
    const ok = validate(req);
    return ok;
  }
}
`
	edgeSyms, err := reg.Extract("src/export_edge.ts", []byte(exportEdgeSrc))
	if err != nil {
		t.Fatalf("Extract(src/export_edge.ts): %v", err)
	}
	if got := findSymbol704(edgeSyms, "handle", symbol.KindMethod); got == nil || got.Line != 6 || got.EndLine != 9 {
		t.Errorf("export_edge.ts: handle got %+v, want 6..9 (all=%+v)", got, edgeSyms)
	}

	defaultClassSrc := `const fallback = { handle: () => 0 };

export default class {
  handle(req: Req) {
    const ok = validate(req);
    return ok;
  }
}
`
	defSyms, err := reg.Extract("src/default_class.ts", []byte(defaultClassSrc))
	if err != nil {
		t.Fatalf("Extract(src/default_class.ts): %v", err)
	}
	if got := findSymbol704(defSyms, "handle", symbol.KindMethod); got == nil || got.Line != 4 || got.EndLine != 7 {
		t.Errorf("default_class.ts: handle got %+v, want 4..7 (all=%+v)", got, defSyms)
	}

	cjsClassSrc := `const fallback = { handle: () => 0 };

module.exports = class {
  handle(req) {
    const ok = validate(req);
    return ok;
  }
};
`
	cjsSyms, err := reg.Extract("src/cjs_class.js", []byte(cjsClassSrc))
	if err != nil {
		t.Fatalf("Extract(src/cjs_class.js): %v", err)
	}
	if got := findSymbol704(cjsSyms, "handle", symbol.KindMethod); got == nil || got.Line != 4 || got.EndLine != 7 {
		t.Errorf("cjs_class.js: handle got %+v, want 4..7 (all=%+v)", got, cjsSyms)
	}

	dottedNsSrc := `function unrelatedHelper() {
  const foo = () => 1;
  return foo();
}

export namespace Orders.V2 {
  export function foo(orderId: string) {
    const normalized = orderId.trim();
    return normalized;
  }
}
`
	dottedSyms, err := reg.Extract("src/dotted_ns.ts", []byte(dottedNsSrc))
	if err != nil {
		t.Fatalf("Extract(src/dotted_ns.ts): %v", err)
	}
	if got := findSymbol704(dottedSyms, "foo", symbol.KindFunction); got == nil || got.Line != 7 || got.EndLine != 10 {
		t.Errorf("dotted_ns.ts: foo got %+v, want 7..10 (all=%+v)", got, dottedSyms)
	}
}
