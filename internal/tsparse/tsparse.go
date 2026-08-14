// Package tsparse constructs tree-sitter parsers that give up on a single file
// instead of running unbounded.
package tsparse

import (
	"time"

	"github.com/odvcencio/gotreesitter"
)

// DefaultTimeout bounds one file's parse.
//
// gotreesitter caps a parse's memory (512MB by default, see
// GOT_PARSE_MEMORY_BUDGET_MB) but not its running time: SetTimeoutMicros(0)
// disables the check and Parser.reset leaves it at 0. Its GLR stack merging can
// spin in node-equivalence lookups without allocating past that budget, which
// is how a single input held a core for 30 hours. The timeout is tested once
// per iteration of the parse loop, so it ends that spin.
//
// The value is deliberately loose. This is a backstop for a parse that will
// never finish, not a latency target, and it is wall-clock — so it has to stay
// clear of legitimate large files on a slow or loaded machine. At 5s it cut off
// real parses in graph's *_cflow_test.go large-input cases once -race slowed
// them down (8.0s and 5.1s). Those tests are the guard against setting this too
// tight; if they start failing, raise this rather than weaken them.
const DefaultTimeout = 30 * time.Second

// NewParser returns a parser for lang that stops after DefaultTimeout.
//
// A parse that hits the timeout returns a partial tree rather than an error —
// callers that care can check Tree.ParseStoppedEarly.
func NewParser(lang *gotreesitter.Language) *gotreesitter.Parser {
	parser := gotreesitter.NewParser(lang)
	parser.SetTimeoutMicros(uint64(DefaultTimeout / time.Microsecond))
	return parser
}
