// Package timefilter provides time parsing for relative and absolute duration filters.
//
// The Parse function accepts three input formats, tried in order (first match wins):
//   - RFC3339 absolute timestamp (e.g., "2026-05-04T12:00:00Z")
//   - Go-style duration (e.g., "720h", "30m")
//   - Humanish relative duration (e.g., "30d", "1w", "2mo", "1y")
//
// For relative durations (Go and humanish), the function validates that the parsed
// duration is strictly positive (> 0) before computing the cutoff time as now.Add(-d).
// Negative or zero durations are rejected with an explicit error. This guard prevents
// silent production of future cutoff times (e.g., -720h would compute now + 720h).
// RFC3339 inputs are NOT subjected to this check — an agent passing a future RFC3339
// timestamp is making an explicit choice.
//
// The now parameter is injected (not time.Now() inside the package) to ensure
// determinism in tests.
package timefilter

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseError is returned when input cannot be parsed as a valid timestamp or duration.
// The caller wraps this error with the parameter name via fmt.Errorf("%s: %w", paramName, err).
type ParseError struct {
	Input  string
	Reason string
}

// Error returns a formatted error message including the offending input and reason.
func (e *ParseError) Error() string {
	return fmt.Sprintf("invalid timestamp or duration %q: %s", e.Input, e.Reason)
}

// Parse accepts input as RFC3339, Go-style duration, or humanish relative duration.
// Returns a time.Time representing the cutoff (absolute time), or an error.
// Relative durations are subtracted from now.
func Parse(input string, now time.Time) (time.Time, error) {
	// Trim leading/trailing whitespace
	input = strings.TrimSpace(input)

	// Reject empty string
	if input == "" {
		return time.Time{}, &ParseError{
			Input:  input,
			Reason: "empty input",
		}
	}

	// Try RFC3339 first
	t, err := time.Parse(time.RFC3339, input)
	if err == nil {
		return t, nil
	}

	// Try Go-style duration
	d, err := time.ParseDuration(input)
	if err == nil {
		// Validate positive (> 0)
		if d <= 0 {
			return time.Time{}, &ParseError{
				Input:  input,
				Reason: fmt.Sprintf("duration must be positive, got %v", d),
			}
		}
		return now.Add(-d), nil
	}

	// Try humanish relative duration
	t, err = parseHumanish(input, now)
	if err == nil {
		return t, nil
	}

	// All parsing failed
	return time.Time{}, &ParseError{
		Input:  input,
		Reason: "not a valid RFC3339 timestamp, Go duration, or humanish duration (e.g., '30d', '1w')",
	}
}

// parseHumanish parses humanish durations like "30d", "1w", "2mo", "1y".
// Units: s, m, h, d, w, mo (30 days), y (365 days).
// Returns the cutoff time (now - parsed duration) or an error.
func parseHumanish(input string, now time.Time) (time.Time, error) {
	// Regex: capture <number><unit> where unit is case-insensitive
	// Match: optional sign, digits, optional decimal, then letters
	re := regexp.MustCompile(`^(-?)(\d+(?:\.\d+)?)(s|m|h|d|w|mo|y)$`)
	matches := re.FindStringSubmatch(strings.ToLower(input))
	if matches == nil {
		return time.Time{}, &ParseError{
			Input:  input,
			Reason: "does not match pattern <number><unit> where unit is s/m/h/d/w/mo/y",
		}
	}

	signStr := matches[1]
	numStr := matches[2]
	unitStr := matches[3]

	// Parse number
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return time.Time{}, &ParseError{
			Input:  input,
			Reason: fmt.Sprintf("invalid number: %v", err),
		}
	}

	// Apply sign
	if signStr == "-" {
		num = -num
	}

	// Map unit to duration
	var baseDuration time.Duration
	switch unitStr {
	case "s":
		baseDuration = time.Second
	case "m":
		baseDuration = time.Minute
	case "h":
		baseDuration = time.Hour
	case "d":
		baseDuration = 24 * time.Hour
	case "w":
		baseDuration = 7 * 24 * time.Hour
	case "mo":
		baseDuration = 30 * 24 * time.Hour
	case "y":
		baseDuration = 365 * 24 * time.Hour
	default:
		return time.Time{}, &ParseError{
			Input:  input,
			Reason: fmt.Sprintf("unknown unit: %s", unitStr),
		}
	}

	// Compute duration
	d := time.Duration(float64(baseDuration) * num)

	// Validate positive (> 0)
	if d <= 0 {
		return time.Time{}, &ParseError{
			Input:  input,
			Reason: fmt.Sprintf("duration must be positive, got %v", d),
		}
	}

	return now.Add(-d), nil
}

// Is reports whether err is a ParseError.
func Is(err error) bool {
	var pe *ParseError
	return errors.As(err, &pe)
}
