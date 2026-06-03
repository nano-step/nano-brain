package search_test

import (
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/search"
)

func TestParseTimeRangeFilter(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		createdAfter   string
		createdBefore  string
		updatedAfter   string
		updatedBefore  string
		expectFilter   bool
		expectError    bool
		expectErrParam string
		expectErrValue string
	}{
		{
			name:         "all empty returns nil filter",
			expectFilter: false,
			expectError:  false,
		},
		{
			name:           "created_after RFC3339",
			createdAfter:   "2026-05-01T00:00:00Z",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:           "updated_after relative duration 30d",
			updatedAfter:   "30d",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:           "created_before Go duration 720h",
			createdBefore:  "720h",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:           "updated_before relative duration 1w",
			updatedBefore:  "1w",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:           "all four filters set",
			createdAfter:   "2026-05-01T00:00:00Z",
			createdBefore:  "2026-06-01T00:00:00Z",
			updatedAfter:   "30d",
			updatedBefore:  "7d",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:            "invalid created_after returns error",
			createdAfter:    "banana",
			expectFilter:    false,
			expectError:     true,
			expectErrParam:  "created_after",
			expectErrValue:  "banana",
		},
		{
			name:            "date-only (no zone) returns error",
			createdAfter:    "2026-05-04",
			expectFilter:    false,
			expectError:     true,
			expectErrParam:  "created_after",
			expectErrValue:  "2026-05-04",
		},
		{
			name:            "negative duration returns error",
			updatedAfter:    "-30d",
			expectFilter:    false,
			expectError:     true,
			expectErrParam:  "updated_after",
			expectErrValue:  "-30d",
		},
		{
			name:            "zero duration returns error",
			updatedBefore:   "0d",
			expectFilter:    false,
			expectError:     true,
			expectErrParam:  "updated_before",
			expectErrValue:  "0d",
		},
		{
			name:            "negative Go duration returns error",
			createdBefore:   "-720h",
			expectFilter:    false,
			expectError:     true,
			expectErrParam:  "created_before",
			expectErrValue:  "-720h",
		},

		{
			name:            "invalid updated_after stops at first error",
			createdAfter:    "2026-05-01T00:00:00Z",
			updatedAfter:    "bad",
			expectFilter:    false,
			expectError:     true,
			expectErrParam:  "updated_after",
			expectErrValue:  "bad",
		},
		{
			name:           "relative duration 1w",
			updatedAfter:   "1w",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:           "relative duration 2mo",
			createdAfter:   "2mo",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:           "relative duration 1y",
			updatedBefore:  "1y",
			expectFilter:   true,
			expectError:    false,
		},
		{
			name:           "case insensitivity for humanish units",
			updatedAfter:   "30D",
			expectFilter:   true,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, paramName, rawValue, err := search.ParseTimeRangeFilter(
				now,
				tt.createdAfter,
				tt.createdBefore,
				tt.updatedAfter,
				tt.updatedBefore,
			)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if paramName != tt.expectErrParam {
					t.Errorf("expected param %q, got %q", tt.expectErrParam, paramName)
				}
				if rawValue != tt.expectErrValue {
					t.Errorf("expected raw value %q, got %q", tt.expectErrValue, rawValue)
				}
				if filter != nil {
					t.Errorf("expected nil filter on error, got %+v", filter)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectFilter {
				if filter == nil {
					t.Fatalf("expected non-nil filter, got nil")
				}
				// Verify raw strings are preserved
				if tt.createdAfter != "" && filter.CreatedAfterRaw != tt.createdAfter {
					t.Errorf("CreatedAfterRaw not preserved: expected %q, got %q", tt.createdAfter, filter.CreatedAfterRaw)
				}
				if tt.createdBefore != "" && filter.CreatedBeforeRaw != tt.createdBefore {
					t.Errorf("CreatedBeforeRaw not preserved: expected %q, got %q", tt.createdBefore, filter.CreatedBeforeRaw)
				}
				if tt.updatedAfter != "" && filter.UpdatedAfterRaw != tt.updatedAfter {
					t.Errorf("UpdatedAfterRaw not preserved: expected %q, got %q", tt.updatedAfter, filter.UpdatedAfterRaw)
				}
				if tt.updatedBefore != "" && filter.UpdatedBeforeRaw != tt.updatedBefore {
					t.Errorf("UpdatedBeforeRaw not preserved: expected %q, got %q", tt.updatedBefore, filter.UpdatedBeforeRaw)
				}
			} else {
				if filter != nil {
					t.Errorf("expected nil filter, got %+v", filter)
				}
			}
		})
	}
}

func TestParseTimeRangeFilter_RawStringPreservation(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	filter, _, _, err := search.ParseTimeRangeFilter(
		now,
		"30d",
		"7d",
		"1w",
		"2mo",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if filter.CreatedAfterRaw != "30d" {
		t.Errorf("CreatedAfterRaw not preserved: expected 30d, got %q", filter.CreatedAfterRaw)
	}
	if filter.CreatedBeforeRaw != "7d" {
		t.Errorf("CreatedBeforeRaw not preserved: expected 7d, got %q", filter.CreatedBeforeRaw)
	}
	if filter.UpdatedAfterRaw != "1w" {
		t.Errorf("UpdatedAfterRaw not preserved: expected 1w, got %q", filter.UpdatedAfterRaw)
	}
	if filter.UpdatedBeforeRaw != "2mo" {
		t.Errorf("UpdatedBeforeRaw not preserved: expected 2mo, got %q", filter.UpdatedBeforeRaw)
	}
}

func TestParseTimeRangeFilter_RawStringWithWhitespace(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	filter, _, _, err := search.ParseTimeRangeFilter(
		now,
		"  2026-05-01T00:00:00Z  ",
		"",
		"",
		"",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if filter.CreatedAfterRaw != "2026-05-01T00:00:00Z" {
		t.Errorf("CreatedAfterRaw should store trimmed value: expected '2026-05-01T00:00:00Z', got %q", filter.CreatedAfterRaw)
	}
}
