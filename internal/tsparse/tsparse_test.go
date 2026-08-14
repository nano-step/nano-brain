package tsparse

import (
	"testing"

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
