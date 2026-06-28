package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newVueSFCExtractor(t *testing.T) *graph.VueSFCExtractor {
	t.Helper()
	ex, err := graph.NewVueSFCExtractor()
	if err != nil {
		t.Fatalf("NewVueSFCExtractor: %v", err)
	}
	return ex
}

func TestVueSFCExtractor_Supports(t *testing.T) {
	ex := newVueSFCExtractor(t)
	if !ex.Supports(".vue") {
		t.Error("should support .vue")
	}
	for _, ext := range []string{".ts", ".js", ".go", ".py", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

func TestVueSFCExtractor_ExtractEdges_BasicScript(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template>
  <p>Hello</p>
</template>

<script setup lang="ts">
import MyComponent from './MyComponent.vue'
import { ref } from 'vue'

const count = ref(0)

function increment() {
  count.value++
  console.log('clicked')
}
</script>
`)

	edges, err := ex.ExtractEdges("src/App.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	var contains, imports, calls []graph.Edge
	for _, e := range edges {
		switch e.Kind {
		case graph.EdgeContains:
			contains = append(contains, e)
		case graph.EdgeImports:
			imports = append(imports, e)
		case graph.EdgeCalls:
			calls = append(calls, e)
		}
	}

	if len(contains) < 2 {
		t.Errorf("expected >=2 contains edges, got %d: %v", len(contains), contains)
	}

	if len(imports) < 2 {
		t.Errorf("expected >=2 import edges, got %d: %v", len(imports), imports)
	}

	if len(calls) == 0 {
		t.Errorf("expected >=1 call edge, got %d", len(calls))
	}
}

func TestVueSFCExtractor_ComponentDetection(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template><MyComponent /></template>

<script setup lang="ts">
import MyComponent from './components/MyComponent.vue'
import Button from '../shared/Button.vue'
import { computed } from 'vue'

const label = computed(() => 'hello')
</script>
`)

	edges, err := ex.ExtractEdges("src/Page.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	var vueImports []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeImports && e.Metadata != nil {
			if comp, ok := e.Metadata["component"].(bool); ok && comp {
				vueImports = append(vueImports, e)
			}
		}
	}

	if len(vueImports) != 2 {
		t.Errorf("expected 2 component imports with metadata, got %d: %v", len(vueImports), vueImports)
	}

	for _, imp := range vueImports {
		if imp.TargetNode != "./components/MyComponent.vue" && imp.TargetNode != "../shared/Button.vue" {
			t.Errorf("unexpected component import target: %s", imp.TargetNode)
		}
	}
}

func TestVueSFCExtractor_LineNumbers(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template><p>Hi</p></template>

<script setup lang="ts">
import Foo from './Foo.vue'

function hello() {
  console.log('world')
}
</script>
`)

	edges, err := ex.ExtractEdges("test.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range edges {
		if e.Line == 0 {
			t.Errorf("edge has zero Line: kind=%s target=%s", e.Kind, e.TargetNode)
		}
	}

	for _, e := range edges {
		if e.Language != "vue" {
			t.Errorf("expected Language=vue, got %q", e.Language)
		}
	}

	for _, e := range edges {
		if e.SourceFile != "test.vue" {
			t.Errorf("expected SourceFile=test.vue, got %q", e.SourceFile)
		}
	}
}

func TestVueSFCExtractor_EmptyScript(t *testing.T) {
	ex := newVueSFCExtractor(t)

	edges, err := ex.ExtractEdges("empty.vue", []byte(`<template><p>Empty</p></template>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from file without script block, got %d: %v", len(edges), edges)
	}
}

func TestVueSFCExtractor_EmptyScriptBlock(t *testing.T) {
	ex := newVueSFCExtractor(t)

	edges, err := ex.ExtractEdges("empty-script.vue", []byte(`<template><p>Hi</p></template>
<script setup lang="ts">
</script>
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from empty script block, got %d: %v", len(edges), edges)
	}
}

func TestVueSFCExtractor_MalformedSFC(t *testing.T) {
	ex := newVueSFCExtractor(t)

	edges, err := ex.ExtractEdges("malformed.vue", []byte(`<template><p>Hi</p>
<script setup lang="ts">
import Foo from './Foo.vue'
function broken( { return }
</script>
`))
	if err != nil {
		t.Fatal("should not error on malformed SFC:", err)
	}
	_ = edges
}

func TestVueSFCExtractor_CallEdges(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template><button @click="handleClick">Click</button></template>

<script setup lang="ts">
import { ref } from 'vue'

const count = ref(0)

function handleClick() {
  count.value++
  saveData()
}

function saveData() {
  console.log('saved')
}
</script>
`)

	edges, err := ex.ExtractEdges("src/Button.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	var callEdges []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			callEdges = append(callEdges, e)
		}
	}

	if len(callEdges) == 0 {
		t.Error("expected call edges")
	}

	foundSaveData := false
	for _, e := range callEdges {
		if e.TargetNode == "saveData" {
			foundSaveData = true
		}
	}
	if !foundSaveData {
		t.Error("expected call edge to saveData")
	}
}

func TestVueSFCExtractor_RequireImport(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template><p>Hi</p></template>

<script>
const utils = require('./utils')
const MyComp = require('./MyComp.vue')
</script>
`)

	edges, err := ex.ExtractEdges("test.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	var imports []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeImports {
			imports = append(imports, e)
		}
	}

	foundUtils := false
	foundComp := false
	for _, imp := range imports {
		if imp.TargetNode == "./utils" {
			foundUtils = true
		}
		if imp.TargetNode == "./MyComp.vue" {
			foundComp = true
			if imp.Metadata == nil || imp.Metadata["component"] != true {
				t.Error("expected component metadata on .vue require import")
			}
		}
	}
	if !foundUtils {
		t.Error("expected require('./utils') import edge")
	}
	if !foundComp {
		t.Error("expected require('./MyComp.vue') import edge")
	}
}

func TestVueSFCExtractor_ContainsEdge(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template><p>Hi</p></template>

<script setup lang="ts">
import { ref } from 'vue'

const name = ref('world')

class Greeter {
  greet() {
    return 'hello'
  }
}

interface Props {
  title: string
}

type Status = 'idle' | 'loading'

enum Mode {
  Read = 0,
  Write = 1,
}
</script>
`)

	edges, err := ex.ExtractEdges("test.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	var contains []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeContains {
			contains = append(contains, e)
		}
	}

	if len(contains) < 4 {
		t.Errorf("expected >=4 contains edges (name, Greeter, Props, Status, Mode), got %d: %v", len(contains), contains)
	}

	names := map[string]bool{}
	for _, e := range contains {
		parts := e.TargetNode
		if idx := len("test.vue::"); idx < len(parts) {
			names[parts[idx:]] = true
		}
	}

	for _, expected := range []string{"name", "Greeter", "Props", "Status", "Mode"} {
		if !names[expected] {
			t.Errorf("expected contains edge for %q", expected)
		}
	}
}

func TestVueSFCExtractor_NonVueFile(t *testing.T) {
	ex := newVueSFCExtractor(t)
	edges, err := ex.ExtractEdges("test.ts", []byte(`import { ref } from 'vue'`))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges for non-.vue file, got %d", len(edges))
	}
}

func TestVueSFCExtractor_JavaScriptScriptBlock(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template><p>JS</p></template>

<script>
import Foo from './Foo.vue'

function init() {
  setup()
}
</script>
`)

	edges, err := ex.ExtractEdges("js.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	var imports []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeImports {
			imports = append(imports, e)
		}
	}

	found := false
	for _, imp := range imports {
		if imp.TargetNode == "./Foo.vue" {
			found = true
		}
	}
	if !found {
		t.Error("expected import edge for ./Foo.vue in JS script block")
	}

	var calls []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			calls = append(calls, e)
		}
	}
	if len(calls) == 0 {
		t.Error("expected call edge inside init function")
	}
}

func TestVueSFCExtractor_MultipleScriptBlocks(t *testing.T) {
	ex := newVueSFCExtractor(t)
	src := []byte(`<template><p>Multi</p></template>

<script>
import Comp1 from './Comp1.vue'
</script>

<script setup lang="ts">
import Comp2 from './Comp2.vue'
</script>
`)

	edges, err := ex.ExtractEdges("multi.vue", src)
	if err != nil {
		t.Fatal(err)
	}

	var imports []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeImports {
			imports = append(imports, e)
		}
	}

	found1 := false
	found2 := false
	for _, imp := range imports {
		if imp.TargetNode == "./Comp1.vue" {
			found1 = true
		}
		if imp.TargetNode == "./Comp2.vue" {
			found2 = true
		}
	}
	if !found1 {
		t.Error("expected import for Comp1.vue from <script> block")
	}
	if !found2 {
		t.Error("expected import for Comp2.vue from <script setup> block")
	}
}
