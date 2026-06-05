package search

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Pre-compiled regex patterns for temporal intent detection
var (
	// "last 7 days", "past 30 days", "last 2 weeks", "past 3 months"
	lastPastPattern = regexp.MustCompile(`(?i)\b(?:last|past)\s+(\d+)\s+(day|week|month)s?\b`)

	// "this week" (Monday of current week)
	thisWeekPattern = regexp.MustCompile(`(?i)\bthis\s+week\b`)

	// "today"
	todayPattern = regexp.MustCompile(`(?i)\btoday\b`)

	// "yesterday"
	yesterdayPattern = regexp.MustCompile(`(?i)\byesterday\b`)

	// "recently", "recent"
	recentPattern = regexp.MustCompile(`(?i)\b(?:recent|recently)\b`)
)

// TemporalHint represents detected temporal intent from a query.
type TemporalHint struct {
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

// DetectTemporalIntent analyzes a query string for temporal cues and returns a hint.
// Returns nil if no temporal cues are found.
func DetectTemporalIntent(query string) *TemporalHint {
	now := time.Now().UTC()

	// Check for "last/past N days/weeks/months"
	if matches := lastPastPattern.FindStringSubmatch(query); matches != nil {
		count, _ := strconv.Atoi(matches[1])
		unit := strings.ToLower(matches[2])

		var after time.Time
		switch unit {
		case "day":
			after = now.AddDate(0, 0, -count)
		case "week":
			after = now.AddDate(0, 0, -count*7)
		case "month":
			after = now.AddDate(0, -count, 0)
		}

		return &TemporalHint{
			CreatedAfter: &after,
		}
	}

	// Check for "this week" (Monday of current week to now)
	if thisWeekPattern.MatchString(query) {
		// Calculate Monday of current week
		weekday := int(now.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}
		daysFromMonday := weekday - 1
		monday := now.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)

		return &TemporalHint{
			CreatedAfter: &monday,
		}
	}

	// Check for "today" (start of today to now)
	if todayPattern.MatchString(query) {
		startOfToday := now.Truncate(24 * time.Hour)

		return &TemporalHint{
			CreatedAfter: &startOfToday,
		}
	}

	// Check for "yesterday" (start of yesterday to start of today)
	if yesterdayPattern.MatchString(query) {
		startOfToday := now.Truncate(24 * time.Hour)
		startOfYesterday := startOfToday.AddDate(0, 0, -1)

		return &TemporalHint{
			CreatedAfter:  &startOfYesterday,
			CreatedBefore: &startOfToday,
		}
	}

	// Check for "recently"/"recent" (last 7 days)
	if recentPattern.MatchString(query) {
		sevenDaysAgo := now.AddDate(0, 0, -7)

		return &TemporalHint{
			CreatedAfter: &sevenDaysAgo,
		}
	}

	return nil
}
