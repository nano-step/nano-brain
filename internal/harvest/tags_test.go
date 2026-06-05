package harvest

import (
	"reflect"
	"sort"
	"testing"
)

func TestInferSemanticTags(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		title    string
		wantTags []string
	}{
		{
			name:     "bug fix in title",
			title:    "Fix authentication bug",
			content:  "Updated the auth flow",
			wantTags: []string{"bug-fix"},
		},
		{
			name:     "feature in content",
			title:    "Update",
			content:  "Implemented new feature for user management",
			wantTags: []string{"feature"},
		},
		{
			name:     "refactor",
			title:    "Refactor database layer",
			content:  "Cleaned up code",
			wantTags: []string{"refactor"},
		},
		{
			name:     "documentation",
			title:    "Update README",
			content:  "Added documentation for API",
			wantTags: []string{"docs"},
		},
		{
			name:     "chore",
			title:    "chore: bump dependencies",
			content:  "Upgraded to latest versions",
			wantTags: []string{"chore"},
		},
		{
			name:     "multiple tags",
			title:    "Fix bug and add feature",
			content:  "Resolved crash and implemented new functionality",
			wantTags: []string{"bug-fix", "feature"},
		},
		{
			name:     "no matches",
			title:    "General update",
			content:  "Made some changes",
			wantTags: []string{},
		},
		{
			name:     "case insensitive",
			title:    "FIX BUG",
			content:  "FEATURE ADDED",
			wantTags: []string{"bug-fix", "feature"},
		},
		{
			name:     "all tags",
			title:    "fix bug feat refactor docs chore",
			content:  "comprehensive update",
			wantTags: []string{"bug-fix", "chore", "docs", "feature", "refactor"},
		},
		{
			name:     "patch keyword",
			title:    "patch security issue",
			content:  "",
			wantTags: []string{"bug-fix"},
		},
		{
			name:     "hotfix keyword",
			title:    "hotfix production crash",
			content:  "",
			wantTags: []string{"bug-fix"},
		},
		{
			name:     "introduce keyword",
			title:    "introduce new API",
			content:  "",
			wantTags: []string{"feature"},
		},
		{
			name:     "cleanup with space",
			title:    "clean up old code",
			content:  "",
			wantTags: []string{"refactor"},
		},
		{
			name:     "simplify keyword",
			title:    "simplify authentication",
			content:  "",
			wantTags: []string{"refactor"},
		},
		{
			name:     "ci config",
			title:    "Update CI configuration",
			content:  "",
			wantTags: []string{"chore"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferSemanticTags(tt.content, tt.title)

			sort.Strings(got)
			sort.Strings(tt.wantTags)

			if !reflect.DeepEqual(got, tt.wantTags) {
				t.Errorf("InferSemanticTags() = %v, want %v", got, tt.wantTags)
			}
		})
	}
}
