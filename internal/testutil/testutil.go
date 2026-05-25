// Package testutil provides testing utilities.
package testutil

import "github.com/rs/zerolog"

func NopLogger() zerolog.Logger {
	return zerolog.Nop()
}
