package search

import (
	"fmt"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/timefilter"
)

// ParseTimeRangeFilter parses four optional raw time-range strings into a
// *TimeRangeFilter. Returns nil if all four inputs are empty (omit-all path).
// On parse error, returns the offending parameter name, the raw input, and
// the underlying error so callers can build surface-appropriate error responses.
//
// `now` is injected for determinism (tests / handlers should pass time.Now().UTC()).
func ParseTimeRangeFilter(
	now time.Time,
	createdAfter, createdBefore, updatedAfter, updatedBefore string,
) (filter *TimeRangeFilter, paramName, rawValue string, err error) {
	// Trim whitespace on all inputs
	createdAfter = strings.TrimSpace(createdAfter)
	createdBefore = strings.TrimSpace(createdBefore)
	updatedAfter = strings.TrimSpace(updatedAfter)
	updatedBefore = strings.TrimSpace(updatedBefore)

	// Omit-all path: if all inputs are empty, return nil filter
	if createdAfter == "" && createdBefore == "" && updatedAfter == "" && updatedBefore == "" {
		return nil, "", "", nil
	}

	filter = &TimeRangeFilter{
		CreatedAfterRaw:  createdAfter,
		CreatedBeforeRaw: createdBefore,
		UpdatedAfterRaw:  updatedAfter,
		UpdatedBeforeRaw: updatedBefore,
	}

	// Parse each non-empty filter. First error wins.
	if createdAfter != "" {
		t, parseErr := timefilter.Parse(createdAfter, now)
		if parseErr != nil {
			return nil, "created_after", createdAfter, fmt.Errorf("invalid created_after: %w", parseErr)
		}
		filter.CreatedAfter = &t
	}

	if createdBefore != "" {
		t, parseErr := timefilter.Parse(createdBefore, now)
		if parseErr != nil {
			return nil, "created_before", createdBefore, fmt.Errorf("invalid created_before: %w", parseErr)
		}
		filter.CreatedBefore = &t
	}

	if updatedAfter != "" {
		t, parseErr := timefilter.Parse(updatedAfter, now)
		if parseErr != nil {
			return nil, "updated_after", updatedAfter, fmt.Errorf("invalid updated_after: %w", parseErr)
		}
		filter.UpdatedAfter = &t
	}

	if updatedBefore != "" {
		t, parseErr := timefilter.Parse(updatedBefore, now)
		if parseErr != nil {
			return nil, "updated_before", updatedBefore, fmt.Errorf("invalid updated_before: %w", parseErr)
		}
		filter.UpdatedBefore = &t
	}

	return filter, "", "", nil
}
