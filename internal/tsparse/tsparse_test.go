package tsparse

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// The whole point of this package is that the parser comes back bounded. A
// parser with no timeout is what held a core and multiple GB of heap, and
// gotreesitter's own default is 0 (unbounded), so an unset timeout is silent.
func TestNewParserIsBounded(t *testing.T) {
	parser := NewParser(grammars.GoLanguage())

	if got := parser.TimeoutMicros(); got == 0 {
		t.Fatal("NewParser returned a parser with no timeout; a runaway parse would be unbounded")
	}
}

func TestNewInjectionParserIsBounded_703(t *testing.T) {
	raw := gotreesitter.NewInjectionParser()
	ip := NewInjectionParser()
	if ip == nil {
		t.Fatal("NewInjectionParser returned nil")
	}

	// InjectionParser does not export TimeoutMicros(), so verify both the
	// configured DefaultTimeout field (contrasted against raw's zero default)
	// and runtime early-stop behavior under a 1us timeout.
	rawField := reflect.ValueOf(raw).Elem().FieldByName("timeoutMicros")
	ipField := reflect.ValueOf(ip).Elem().FieldByName("timeoutMicros")
	if !rawField.IsValid() || !ipField.IsValid() {
		t.Fatal("gotreesitter.InjectionParser no longer has timeoutMicros field; update boundedness check")
	}
	if rawField.Uint() != 0 {
		t.Fatalf("expected raw InjectionParser default timeout 0, got %d", rawField.Uint())
	}
	wantMicros := uint64(DefaultTimeout / time.Microsecond)
	if got := ipField.Uint(); got != wantMicros {
		t.Fatalf("NewInjectionParser timeoutMicros = %d, want %d", got, wantMicros)
	}

	ip.RegisterLanguage("vue", grammars.VueLanguage())
	ip.RegisterLanguage("typescript", grammars.TypescriptLanguage())
	if err := ip.RegisterInjectionQuery("vue", `(script_element (raw_text) @injection.content) (#set! injection.language "typescript")`); err != nil {
		t.Fatalf("RegisterInjectionQuery: %v", err)
	}
	ip.SetTimeoutMicros(1)

	var b strings.Builder
	b.WriteString("<script setup lang=\"ts\">\n")
	for i := 0; i < 2000; i++ {
		b.WriteString("export const fn = (x: number): number => x + 1;\n")
	}
	b.WriteString("</script>\n")

	res, err := ip.Parse([]byte(b.String()), "vue")
	if err != nil {
		t.Fatalf("Parse under 1us timeout returned error: %v", err)
	}
	defer func() {
		if res == nil {
			return
		}
		if res.Tree != nil {
			res.Tree.Release()
		}
		for _, inj := range res.Injections {
			if inj.Tree != nil {
				inj.Tree.Release()
			}
		}
	}()

	stopped := res.Tree != nil && res.Tree.ParseStoppedEarly()
	for _, inj := range res.Injections {
		if inj.Tree != nil && inj.Tree.ParseStoppedEarly() {
			stopped = true
		}
	}
	if !stopped {
		t.Fatal("expected InjectionParser with 1us timeout to stop early on large Vue SFC")
	}
}
