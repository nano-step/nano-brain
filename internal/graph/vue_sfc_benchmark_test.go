package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func loadFixture(t testing.TB, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "vue-fixture", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

func newTestVueExtractor(t testing.TB) *graph.VueSFCExtractor {
	t.Helper()
	ex, err := graph.NewVueSFCExtractor()
	if err != nil {
		t.Fatalf("NewVueSFCExtractor: %v", err)
	}
	return ex
}

func BenchmarkVueSFCExtractor_ExtractEdges_Small(b *testing.B) {
	ex := newTestVueExtractor(b)
	content := loadFixture(b, "small-counter.vue")
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ex.ExtractEdges("small-counter.vue", content); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVueSFCExtractor_ExtractEdges_Medium(b *testing.B) {
	ex := newTestVueExtractor(b)
	content := loadFixture(b, "medium-component.vue")
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ex.ExtractEdges("medium-component.vue", content); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVueSFCExtractor_ExtractEdges_Large(b *testing.B) {
	ex := newTestVueExtractor(b)
	content := loadFixture(b, "large-page.vue")
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ex.ExtractEdges("large-page.vue", content); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVueSFCExtractor_ExtractEdges_Empty(b *testing.B) {
	ex := newTestVueExtractor(b)
	content := loadFixture(b, "template-only.vue")
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ex.ExtractEdges("template-only.vue", content); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVueSFCExtractor_ExtractEdges_ComponentHeavy(b *testing.B) {
	ex := newTestVueExtractor(b)
	content := loadFixture(b, "component-heavy.vue")
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ex.ExtractEdges("component-heavy.vue", content); err != nil {
			b.Fatal(err)
		}
	}
}
