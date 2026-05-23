package bench

import (
	"math"
	"testing"
)

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestPrecisionAtK(t *testing.T) {
	tests := []struct {
		name string
		qrs  []QueryResult
		k    int
		want float64
	}{
		{
			name: "3 of 5 relevant",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a", "b", "c"},
				ReturnedDocIDs: []string{"a", "x", "b", "y", "c", "z"},
			}},
			k:    5,
			want: 0.6,
		},
		{
			name: "0 of 5 relevant",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a", "b"},
				ReturnedDocIDs: []string{"x", "y", "z", "w", "v"},
			}},
			k:    5,
			want: 0.0,
		},
		{
			name: "5 of 5 relevant",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a", "b", "c", "d", "e"},
				ReturnedDocIDs: []string{"a", "b", "c", "d", "e"},
			}},
			k:    5,
			want: 1.0,
		},
		{
			name: "empty results",
			qrs:  nil,
			k:    5,
			want: 0.0,
		},
		{
			name: "fewer returned than k",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a", "b"},
				ReturnedDocIDs: []string{"a", "b"},
			}},
			k:    5,
			want: 0.4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrecisionAtK(tt.qrs, tt.k)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("PrecisionAtK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecallAtK(t *testing.T) {
	tests := []struct {
		name string
		qrs  []QueryResult
		k    int
		want float64
	}{
		{
			name: "2 of 3 relevant found in top 10",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a", "b", "c"},
				ReturnedDocIDs: []string{"a", "x", "b", "y", "z", "w", "v", "u", "t", "s"},
			}},
			k:    10,
			want: 2.0 / 3.0,
		},
		{
			name: "0 of 3 found",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a", "b", "c"},
				ReturnedDocIDs: []string{"x", "y", "z", "w", "v", "u", "t", "s", "r", "q"},
			}},
			k:    10,
			want: 0.0,
		},
		{
			name: "all found",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a", "b"},
				ReturnedDocIDs: []string{"a", "b", "x", "y"},
			}},
			k:    10,
			want: 1.0,
		},
		{
			name: "empty results",
			qrs:  nil,
			k:    10,
			want: 0.0,
		},
		{
			name: "empty relevant excluded from denominator",
			qrs: []QueryResult{
				{
					RelevantDocIDs: []string{"a"},
					ReturnedDocIDs: []string{"a", "x"},
				},
				{
					RelevantDocIDs: nil,
					ReturnedDocIDs: []string{"x", "y"},
				},
			},
			k:    10,
			want: 1.0,
		},
		{
			name: "all queries have empty relevant",
			qrs: []QueryResult{{
				RelevantDocIDs: nil,
				ReturnedDocIDs: []string{"x"},
			}},
			k:    10,
			want: 0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecallAtK(tt.qrs, tt.k)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("RecallAtK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMeanReciprocalRank(t *testing.T) {
	tests := []struct {
		name string
		qrs  []QueryResult
		want float64
	}{
		{
			name: "first relevant at rank 1",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a"},
				ReturnedDocIDs: []string{"a", "b", "c"},
			}},
			want: 1.0,
		},
		{
			name: "first relevant at rank 3",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a"},
				ReturnedDocIDs: []string{"x", "y", "a", "b"},
			}},
			want: 1.0 / 3.0,
		},
		{
			name: "no relevant found",
			qrs: []QueryResult{{
				RelevantDocIDs: []string{"a"},
				ReturnedDocIDs: []string{"x", "y", "z"},
			}},
			want: 0.0,
		},
		{
			name: "multiple queries averaged",
			qrs: []QueryResult{
				{
					RelevantDocIDs: []string{"a"},
					ReturnedDocIDs: []string{"a", "b"},
				},
				{
					RelevantDocIDs: []string{"c"},
					ReturnedDocIDs: []string{"x", "y", "c"},
				},
			},
			want: (1.0 + 1.0/3.0) / 2.0,
		},
		{
			name: "empty results",
			qrs:  nil,
			want: 0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MeanReciprocalRank(tt.qrs)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("MeanReciprocalRank() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		p      float64
		want   float64
	}{
		{
			name:   "p50 of odd count",
			values: []float64{3, 1, 2, 5, 4},
			p:      50,
			want:   3.0,
		},
		{
			name:   "p95 of 20 values",
			values: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			p:      95,
			want:   19.0,
		},
		{
			name:   "p50 of single value",
			values: []float64{42},
			p:      50,
			want:   42.0,
		},
		{
			name:   "empty",
			values: nil,
			p:      50,
			want:   0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Percentile(tt.values, tt.p)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("Percentile() = %v, want %v", got, tt.want)
			}
		})
	}
}
