package symbol_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/symbol"
)

func TestRubyExtractor_Supports(t *testing.T) {
	e, err := symbol.NewRubySymbolExtractor()
	if err != nil {
		t.Fatal(err)
	}
	if !e.Supports(".rb") {
		t.Error("should support .rb")
	}
	if e.Supports(".py") {
		t.Error("should not support .py")
	}
	if e.Supports(".go") {
		t.Error("should not support .go")
	}
}

func TestRubyExtractor_Methods(t *testing.T) {
	e, err := symbol.NewRubySymbolExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`
def greet(name)
  puts "hello #{name}"
end

def self.from_email(email)
  find_by(email: email)
end
`)
	syms, err := e.Extract("user.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]symbol.Kind{
		"greet":      symbol.KindFunction,
		"from_email": symbol.KindFunction,
	}
	got := make(map[string]symbol.Kind)
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("Ruby method %s: want %s got %s", name, kind, got[name])
		}
	}
}

func TestRubyExtractor_ClassAndModule(t *testing.T) {
	e, err := symbol.NewRubySymbolExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`
class User
  def initialize(name)
    @name = name
  end

  def greet
    puts "hello #{@name}"
  end
end

module Authenticable
  def authenticate
    true
  end
end
`)
	syms, err := e.Extract("app.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]symbol.Kind{
		"User":          symbol.KindType,
		"Authenticable": symbol.KindType,
		"initialize":    symbol.KindFunction,
		"greet":         symbol.KindFunction,
		"authenticate":  symbol.KindFunction,
	}
	got := make(map[string]symbol.Kind)
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("Ruby %s: want %s got %s", name, kind, got[name])
		}
	}
}

func TestRubyExtractor_Constants(t *testing.T) {
	e, err := symbol.NewRubySymbolExtractor()
	if err != nil {
		t.Fatal(err)
	}

	src := []byte(`
module PrintOrderStatus
  STATUS_ORDER_SUBMITTED = "submitted"
  STATUS_ORDER_PAID = "paid"
end

class PrintOrder
  STATUS_ORDER_PRINTING = "printing"
end
`)
	syms, err := e.Extract("app/models/concerns/print_order_status.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]symbol.Kind{
		"PrintOrderStatus":       symbol.KindType,
		"PrintOrder":             symbol.KindType,
		"STATUS_ORDER_SUBMITTED": symbol.KindConst,
		"STATUS_ORDER_PAID":      symbol.KindConst,
		"STATUS_ORDER_PRINTING":  symbol.KindConst,
	}
	got := make(map[string]symbol.Kind)
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("Ruby %s: want %s got %s", name, kind, got[name])
		}
	}
}

func TestRubyExtractor_Language(t *testing.T) {
	e, err := symbol.NewRubySymbolExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("def foo; end")
	syms, err := e.Extract("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Fatal("expected at least one symbol")
	}
	if syms[0].Language != "ruby" {
		t.Errorf("want language ruby, got %s", syms[0].Language)
	}
}

func TestRubyExtractor_FilePath(t *testing.T) {
	e, err := symbol.NewRubySymbolExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("def foo; end")
	syms, err := e.Extract("app/models/user.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Fatal("expected at least one symbol")
	}
	if syms[0].File != "app/models/user.rb" {
		t.Errorf("want file app/models/user.rb, got %s", syms[0].File)
	}
}
