# Research Findings — What Agents Need from nano-brain

## Finding 1: Impact Analysis is #1 Use Case

> "Before modifying a public component, evaluate the impact scope" — ai-dependency-analyzer
> "Know what breaks before you ship" — ImpactTrace

### Agent Workflow (real, from production tools)

1. Agent proposes edit
2. `memory_impact` runs → returns affected files, risk score
3. If HIGH risk → agent splits change
4. If LOW risk → proceeds
5. After edit → runs affected tests only (not full suite)

### Evidence

- **Gortex**: "What breaks if I change this?" is the question ungrounded agents answer worst
- **CodeGraph**: `fn-impact <name>` before every edit
- **Carto**: Agent threw out its own patch after impact analysis showed 83 transitive dependents
- **Latency requirement**: Sub-50ms (at 50ms agent skips, at 0.04ms no reason not to run)

---

## Finding 2: Call Chains > Control Flow

Agents operate at **inter-procedural level** (cross-file), not intra-procedural (within 1 function).

### Evidence

- Out of 15+ production code intelligence tools, only 2 expose CFG as primary tool
- Gortex: "token-budgeted call chains" — not CFG
- trace-mcp comparison: CFG listed as "threat" (niche feature), not standard
- Stack Overflow: "CFG provides finer details into program structure... Call graph gives inter-procedural view"

### When CFG IS used

- Security analysis (taint tracking)
- Compiler-grade rigor (Joern, Narsil-MCP)
- NOT for typical agent workflows (refactoring, feature, bug fix)

### Conclusion

`memory_flowchart` is underutilized. Focus on `memory_impact` and `memory_trace`.

---

## Finding 3: Component Composition > Internal Logic

Every Vue/React-specific tool focuses on **component composition**, not internal control flow.

### Evidence

**React Graph AI**:
- Component render hierarchy
- Props flow
- Hook dependencies
- State propagation
- Next.js boundaries

**vue-harvest**:
- Full interface (props, emits, slots)
- Dependency graph
- Coupling issues detection

**ai-dependency-analyzer**:
- Supports Vue, React, Next.js, NestJS
- `get_file_impact`, `get_downstream_dependencies`, `get_upstream_dependencies`
- "修改一个公共组件前，先评估影响范围" (Before modifying a public component, evaluate impact scope)

**voyager**:
- Interactive dependency diagrams for Vue.js components
- Visualize complex component relationships

### What agents need for Vue components

1. **Import graph** (`.vue` → `.vue`, `.vue` → `.ts`, `.vue` → composable)
2. **Component composition** (`<template>` references to child components)
3. **Props flow** (parent → child, type-checked)
4. **Event/emit flow** (child emit → parent handler)
5. **Slot usage** (parent provides slot → child defines slot)
6. **Store dependencies** (component → Pinia store)
7. **Composable usage** (component → useAuth, useFetch, etc.)
8. **Route → page** mapping (Nuxt/Vue Router)

### What agents DON'T need (for typical workflows)

- Internal template logic (v-if/v-for conditions)
- Reactive state machine tracking
- Function-level call graphs within `<script setup>`

---

## nano-brain Current State

### Gaming-platform workspace (Vue/Nuxt)

- **Avg Latency**: 828ms across 70 queries
- **Avg P@5**: 0.75 (75%)
- **First query cold start**: 3630ms
- **Subsequent queries**: ~783ms

### Code Flow Support

| Layer | Go | Ruby/Rails | JS/TS | Vue/Nuxt |
|-------|----|-----------|-------|----------|
| Route → Handler | ✅ | ✅ | ✅ | ✅ (NuxtExtractor) |
| Call graph | ✅ | ✅ | ✅ | ❌ (missing) |
| CFG | ❌ | ❌ | ✅ | ❌ (missing) |
| Symbols | ✅ | ✅ | ✅ | ❌ (missing) |

### Missing for Vue/Nuxt

- Vue SFC parsing (script/template split)
- Component tree (parent→child, props, emits)
- useFetch/useAsyncData data flow
- Composable usage (useXxx)
- Pinia/Vuex store
- .vue control flow
- Nuxt middleware
- Nuxt plugins
