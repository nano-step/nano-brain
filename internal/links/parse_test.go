package links

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Link
	}{
		{
			name:  "simple_uuid",
			input: "hello [[123e4567-e89b-12d3-a456-426614174000]] world",
			want: []Link{{
				Raw:       "[[123e4567-e89b-12d3-a456-426614174000]]",
				TargetRef: "123e4567-e89b-12d3-a456-426614174000",
				Kind:      KindID,
				Start:     6,
				End:       46,
			}},
		},
		{
			name:  "simple_title",
			input: "see [[Architecture Overview]] for details",
			want: []Link{{
				Raw:       "[[Architecture Overview]]",
				TargetRef: "Architecture Overview",
				Kind:      KindTitle,
				Start:     4,
				End:       29,
			}},
		},
		{
			name:  "mixed",
			input: "A [[Foo]] and [[123e4567-e89b-12d3-a456-426614174000]] and [[Bar]]",
			want: []Link{
				{Raw: "[[Foo]]", TargetRef: "Foo", Kind: KindTitle, Start: 2, End: 9},
				{Raw: "[[123e4567-e89b-12d3-a456-426614174000]]", TargetRef: "123e4567-e89b-12d3-a456-426614174000", Kind: KindID, Start: 14, End: 54},
				{Raw: "[[Bar]]", TargetRef: "Bar", Kind: KindTitle, Start: 59, End: 66},
			},
		},
		{
			name:  "escaped",
			input: `render \[[literal text]] as text`,
			want:  nil,
		},
		{
			name:  "malformed_unterminated",
			input: "[[ unterminated",
			want:  nil,
		},
		{
			name:  "nested_brackets_inner_extracted",
			input: "outer [[inner [[deep]]",
			want: []Link{{
				Raw:       "[[deep]]",
				TargetRef: "deep",
				Kind:      KindTitle,
				Start:     14,
				End:       22,
			}},
		},
		{
			name:  "over_200_chars",
			input: "[[" + strings.Repeat("a", 201) + "]]",
			want:  nil,
		},
		{
			name:  "unicode_title",
			input: "[[日本語 タイトル]]",
			want: []Link{{
				Raw:       "[[日本語 タイトル]]",
				TargetRef: "日本語 タイトル",
				Kind:      KindTitle,
				Start:     0,
				End:       len("[[日本語 タイトル]]"),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i].Raw != tt.want[i].Raw {
					t.Errorf("[%d] Raw = %q, want %q", i, got[i].Raw, tt.want[i].Raw)
				}
				if got[i].TargetRef != tt.want[i].TargetRef {
					t.Errorf("[%d] TargetRef = %q, want %q", i, got[i].TargetRef, tt.want[i].TargetRef)
				}
				if got[i].Kind != tt.want[i].Kind {
					t.Errorf("[%d] Kind = %d, want %d", i, got[i].Kind, tt.want[i].Kind)
				}
				if got[i].Start != tt.want[i].Start {
					t.Errorf("[%d] Start = %d, want %d", i, got[i].Start, tt.want[i].Start)
				}
				if got[i].End != tt.want[i].End {
					t.Errorf("[%d] End = %d, want %d", i, got[i].End, tt.want[i].End)
				}
			}
		})
	}
}
