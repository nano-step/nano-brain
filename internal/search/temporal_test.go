package search

import (
	"testing"
	"time"
)

func TestDetectTemporalIntent(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		wantNil       bool
		checkAfter    bool
		afterDaysAgo  int
		checkBefore   bool
		beforeDaysAgo int
	}{
		{
			name:         "last 7 days",
			query:        "show me bugs from the last 7 days",
			checkAfter:   true,
			afterDaysAgo: 7,
		},
		{
			name:         "past 30 days",
			query:        "past 30 days of commits",
			checkAfter:   true,
			afterDaysAgo: 30,
		},
		{
			name:         "last 2 weeks",
			query:        "last 2 weeks",
			checkAfter:   true,
			afterDaysAgo: 14,
		},
		{
			name:         "past 3 months",
			query:        "past 3 months",
			checkAfter:   true,
			afterDaysAgo: 90,
		},
		{
			name:       "this week",
			query:      "this week's changes",
			checkAfter: true,
		},
		{
			name:         "today",
			query:        "today",
			checkAfter:   true,
			afterDaysAgo: 0,
		},
		{
			name:          "yesterday",
			query:         "yesterday's work",
			checkAfter:    true,
			afterDaysAgo:  1,
			checkBefore:   true,
			beforeDaysAgo: 0,
		},
		{
			name:         "recently",
			query:        "recently added features",
			checkAfter:   true,
			afterDaysAgo: 7,
		},
		{
			name:         "recent",
			query:        "recent bugs",
			checkAfter:   true,
			afterDaysAgo: 7,
		},
		{
			name:    "no temporal cues",
			query:   "find authentication code",
			wantNil: true,
		},
		{
			name:    "empty query",
			query:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := DetectTemporalIntent(tt.query)

			if tt.wantNil {
				if hint != nil {
					t.Errorf("expected nil, got %+v", hint)
				}
				return
			}

			if hint == nil {
				t.Fatal("expected hint, got nil")
			}

			now := time.Now().UTC()

			if tt.checkAfter {
				if hint.CreatedAfter == nil {
					t.Error("expected CreatedAfter, got nil")
				} else {
					if tt.name == "this week" {
						weekday := int(now.Weekday())
						if weekday == 0 {
							weekday = 7
						}
						daysFromMonday := weekday - 1
						expectedMonday := now.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)

						if hint.CreatedAfter.Before(expectedMonday.Add(-1*time.Hour)) ||
							hint.CreatedAfter.After(expectedMonday.Add(1*time.Hour)) {
							t.Errorf("CreatedAfter = %v, want around %v", hint.CreatedAfter, expectedMonday)
						}
					} else if tt.name == "past 3 months" {
						diff := now.Sub(*hint.CreatedAfter).Hours() / 24
						if diff < 88 || diff > 93 {
							t.Errorf("CreatedAfter days ago = %.0f, want between 88-93", diff)
						}
					} else if tt.name == "today" {
						startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
						if !hint.CreatedAfter.Equal(startOfDay) {
							t.Errorf("CreatedAfter = %v, want %v", hint.CreatedAfter, startOfDay)
						}
					} else if tt.name == "yesterday" {
						startOfYesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
						if !hint.CreatedAfter.Equal(startOfYesterday) {
							t.Errorf("CreatedAfter = %v, want %v", hint.CreatedAfter, startOfYesterday)
						}
					} else {
						expected := now.AddDate(0, 0, -tt.afterDaysAgo)
						diff := expected.Sub(*hint.CreatedAfter)
						if diff < -1*time.Hour || diff > 1*time.Hour {
							t.Errorf("CreatedAfter = %v, want around %v (diff: %v)", hint.CreatedAfter, expected, diff)
						}
					}
				}
			}

			if tt.checkBefore {
				if hint.CreatedBefore == nil {
					t.Error("expected CreatedBefore, got nil")
				} else {
					expected := now.AddDate(0, 0, -tt.beforeDaysAgo).Truncate(24 * time.Hour)
					diff := expected.Sub(*hint.CreatedBefore)
					if diff < -1*time.Hour || diff > 1*time.Hour {
						t.Errorf("CreatedBefore = %v, want around %v", hint.CreatedBefore, expected)
					}
				}
			}
		})
	}
}
