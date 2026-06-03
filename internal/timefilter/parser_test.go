package timefilter

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   string
		now     time.Time
		want    time.Time
		wantErr bool
	}{
		// RFC3339 absolute timestamps
		{
			name:    "RFC3339: basic timestamp",
			input:   "2026-05-04T12:00:00Z",
			now:     now,
			want:    time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "RFC3339: with timezone offset",
			input:   "2026-05-04T12:00:00+02:00",
			now:     now,
			want:    time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "RFC3339: future timestamp allowed",
			input:   "2099-01-01T00:00:00Z",
			now:     now,
			want:    time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},

		// Go-style durations
		{
			name:    "Go duration: 720h",
			input:   "720h",
			now:     now,
			want:    now.Add(-720 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Go duration: 30m",
			input:   "30m",
			now:     now,
			want:    now.Add(-30 * time.Minute),
			wantErr: false,
		},
		{
			name:    "Go duration: 30s",
			input:   "30s",
			now:     now,
			want:    now.Add(-30 * time.Second),
			wantErr: false,
		},
		{
			name:    "Go duration: 1h30m composite",
			input:   "1h30m",
			now:     now,
			want:    now.Add(-(1*time.Hour + 30*time.Minute)),
			wantErr: false,
		},

		// Humanish durations
		{
			name:    "Humanish: 30d (days)",
			input:   "30d",
			now:     now,
			want:    now.Add(-30 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Humanish: 1w (week)",
			input:   "1w",
			now:     now,
			want:    now.Add(-7 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Humanish: 2mo (months, ~30d)",
			input:   "2mo",
			now:     now,
			want:    now.Add(-2 * 30 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Humanish: 1y (year, 365d)",
			input:   "1y",
			now:     now,
			want:    now.Add(-365 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Humanish: 1s (second)",
			input:   "1s",
			now:     now,
			want:    now.Add(-time.Second),
			wantErr: false,
		},
		{
			name:    "Humanish: 1h (hour)",
			input:   "1h",
			now:     now,
			want:    now.Add(-time.Hour),
			wantErr: false,
		},

		// Case insensitivity
		{
			name:    "Case: uppercase D",
			input:   "30D",
			now:     now,
			want:    now.Add(-30 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Case: uppercase W",
			input:   "1W",
			now:     now,
			want:    now.Add(-7 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Case: uppercase MO",
			input:   "2MO",
			now:     now,
			want:    now.Add(-2 * 30 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Case: uppercase Y",
			input:   "1Y",
			now:     now,
			want:    now.Add(-365 * 24 * time.Hour),
			wantErr: false,
		},

		// Whitespace handling
		{
			name:    "Whitespace: leading and trailing spaces",
			input:   "  30d  ",
			now:     now,
			want:    now.Add(-30 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "Whitespace: only leading",
			input:   "  30d",
			now:     now,
			want:    now.Add(-30 * 24 * time.Hour),
			wantErr: false,
		},

		// Invalid inputs
		{
			name:    "Invalid: non-numeric text",
			input:   "banana",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Invalid: number without unit",
			input:   "30",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Invalid: unknown unit",
			input:   "30x",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Invalid: trailing garbage",
			input:   "30dxyz",
			now:     now,
			wantErr: true,
		},

		// Date-only rejection (no timezone)
		{
			name:    "Date-only: YYYY-MM-DD rejected",
			input:   "2026-05-04",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Date-only: YYYY/MM/DD rejected",
			input:   "2026/05/04",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Date-only: datetime without zone rejected",
			input:   "2026-05-04T12:00:00",
			now:     now,
			wantErr: true,
		},

		// Empty string rejection
		{
			name:    "Empty: empty string",
			input:   "",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Empty: whitespace only",
			input:   "   ",
			now:     now,
			wantErr: true,
		},

		// Negative Go durations rejected
		{
			name:    "Negative Go: -720h",
			input:   "-720h",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Negative Go: -30m",
			input:   "-30m",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Negative Go: -1h30m",
			input:   "-1h30m",
			now:     now,
			wantErr: true,
		},

		// Zero Go durations rejected
		{
			name:    "Zero Go: 0h",
			input:   "0h",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Zero Go: 0m",
			input:   "0m",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Zero Go: 0s",
			input:   "0s",
			now:     now,
			wantErr: true,
		},

		// Negative humanish durations rejected
		{
			name:    "Negative humanish: -30d",
			input:   "-30d",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Negative humanish: -1w",
			input:   "-1w",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Negative humanish: -2mo",
			input:   "-2mo",
			now:     now,
			wantErr: true,
		},

		// Zero humanish durations rejected
		{
			name:    "Zero humanish: 0d",
			input:   "0d",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Zero humanish: 0w",
			input:   "0w",
			now:     now,
			wantErr: true,
		},
		{
			name:    "Zero humanish: 0mo",
			input:   "0mo",
			now:     now,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.input, tc.now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil {
				// For error cases, just verify the error is a ParseError
				if !Is(err) {
					t.Fatalf("Parse() error type = %T, want ParseError", err)
				}
				// Verify error message includes the offending input (trimmed for whitespace-only inputs).
				trimmed := strings.TrimSpace(tc.input)
				if trimmed != "" && !contains(err.Error(), trimmed) {
					t.Errorf("Parse() error %q does not contain input %q", err.Error(), tc.input)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Errorf("Parse() got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseDeterminism(t *testing.T) {
	// Verify that same input + same now produce identical results
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	result1, err1 := Parse("30d", now)
	result2, err2 := Parse("30d", now)

	if err1 != nil || err2 != nil {
		t.Fatalf("Parse() error = %v, %v", err1, err2)
	}
	if !result1.Equal(result2) {
		t.Errorf("Parse() not deterministic: got %v then %v", result1, result2)
	}
}

// contains checks if needle is a substring of haystack.
func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
