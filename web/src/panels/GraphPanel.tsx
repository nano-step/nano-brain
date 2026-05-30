import React, { useState, useCallback, useEffect, Suspense } from 'react'
import { getCurrentWorkspace } from '../api/workspace'
import { useGraphNeighborhood } from './graph/useGraphNeighborhood'
import { usePositionCache } from './graph/usePositionCache'
import { GraphLegend } from './graph/GraphLegend'
import type { NodeKind, GraphDirection, EdgeType, GraphNode, GraphEdge } from '../api/types'
import type { PositionMap } from './graph/usePositionCache'

const SigmaGraph = React.lazy(() =>
  import('./graph/SigmaGraph').then((m) => ({ default: m.SigmaGraph }))
)

const CODE_EDGE_TYPES: EdgeType[] = ['calls', 'imports', 'contains']
const KNOWLEDGE_EDGE_TYPES: EdgeType[] = ['references']

function GraphSkeleton() {
  return (
    <div
      className="skel"
      style={{
        height: 480,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: 'var(--text-3)',
        fontSize: 12,
      }}
    >
      Loading graph engine…
    </div>
  )
}

function ControlBar({
  mode,
  focus,
  depth,
  direction,
  edgeTypes,
  onModeChange,
  onFocusChange,
  onDepthChange,
  onDirectionChange,
  onEdgeTypeToggle,
  truncated,
  frontierCount,
  nodeCount,
  edgeCount,
}: {
  mode: NodeKind
  focus: string
  depth: number
  direction: GraphDirection
  edgeTypes: EdgeType[]
  onModeChange: (m: NodeKind) => void
  onFocusChange: (v: string) => void
  onDepthChange: (d: number) => void
  onDirectionChange: (d: GraphDirection) => void
  onEdgeTypeToggle: (t: EdgeType) => void
  truncated: boolean
  frontierCount: number
  nodeCount: number
  edgeCount: number
}) {
  const isCode = mode === 'symbol'
  const availableEdgeTypes: EdgeType[] = isCode ? CODE_EDGE_TYPES : KNOWLEDGE_EDGE_TYPES

  return (
    <div className="filter-bar">
      <div style={{ display: 'inline-flex', border: '1px solid var(--border-strong)' }}>
        <button
          className="chip"
          data-active={isCode ? 'true' : 'false'}
          onClick={() => onModeChange('symbol')}
          style={{ border: 0 }}
        >
          Code
        </button>
        <button
          className="chip"
          data-active={!isCode ? 'true' : 'false'}
          onClick={() => onModeChange('doc')}
          style={{ border: 0, borderLeft: '1px solid var(--border-strong)' }}
        >
          Knowledge
        </button>
      </div>

      <input
        className="filter-input"
        placeholder={
          isCode
            ? 'Focus node (function or type)…'
            : 'Enter a memory doc title or ID, or Cmd+K to find one'
        }
        value={focus}
        onChange={(e) => onFocusChange(e.target.value)}
        style={{ minWidth: 260 }}
      />

      <span style={{ color: 'var(--text-3)', fontSize: 11 }}>depth</span>
      {([1, 2, 3, 4, 5] as const).map((d) => (
        <button
          key={d}
          className="chip"
          data-active={depth === d ? 'true' : 'false'}
          onClick={() => onDepthChange(d)}
        >
          {d}
        </button>
      ))}

      <span style={{ color: 'var(--text-3)', fontSize: 11, marginLeft: 8 }}>direction</span>
      {(['in', 'out', 'both'] as GraphDirection[]).map((d) => (
        <button
          key={d}
          className="chip"
          data-active={direction === d ? 'true' : 'false'}
          onClick={() => onDirectionChange(d)}
        >
          {d}
        </button>
      ))}

      <span style={{ color: 'var(--text-3)', fontSize: 11, marginLeft: 8 }}>edges</span>
      {availableEdgeTypes.map((t) => (
        <button
          key={t}
          className="chip"
          data-active={edgeTypes.includes(t) ? 'true' : 'false'}
          onClick={() => onEdgeTypeToggle(t)}
        >
          {t}
        </button>
      ))}

      <span style={{ marginLeft: 'auto', display: 'flex', gap: 6, alignItems: 'center' }}>
        <span className="pill">{nodeCount} nodes</span>
        <span className="pill">{edgeCount} edges</span>
        {truncated ? (
          <span className="pill pill-warn">
            truncated · {frontierCount} frontier
          </span>
        ) : (
          <span className="pill pill-ok">truncated: false</span>
        )}
      </span>
    </div>
  )
}

interface ModeState {
  focus: string
  depth: number
  direction: GraphDirection
  edgeTypes: EdgeType[]
}

const defaultCodeState: ModeState = {
  focus: '',
  depth: 2,
  direction: 'both',
  edgeTypes: ['calls', 'imports', 'contains'],
}

const defaultKnowledgeState: ModeState = {
  focus: '',
  depth: 2,
  direction: 'both',
  edgeTypes: ['references'],
}

export function GraphPanel() {
  const workspace = getCurrentWorkspace()

  const [mode, setMode] = useState<NodeKind>('symbol')
  const [codeState, setCodeState] = useState<ModeState>(defaultCodeState)
  const [knowledgeState, setKnowledgeState] = useState<ModeState>(defaultKnowledgeState)

  const state = mode === 'symbol' ? codeState : knowledgeState
  const setState = mode === 'symbol' ? setCodeState : setKnowledgeState

  const { mutate: fetchNeighborhood, data, isPending, isError, error } = useGraphNeighborhood({
    workspace,
  })

  const positionCache = usePositionCache({
    workspace: workspace ?? '',
    mode,
    focus: state.focus,
    depth: state.depth,
    direction: state.direction,
    edge_types: state.edgeTypes,
  })

  const [cachedPositions, setCachedPositions] = useState<PositionMap | null>(null)

  useEffect(() => {
    if (!state.focus || !workspace) return
    setCachedPositions(positionCache.read())
    fetchNeighborhood({
      focus: state.focus,
      depth: state.depth,
      direction: state.direction,
      edge_types: state.edgeTypes,
      node_kind: mode,
    })
    // positionCache.read is stable (memoized by useCallback); fetchNeighborhood from useMutation
    // is intentionally excluded — it's a stable reference that doesn't need to be in deps.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.focus, state.depth, state.direction, state.edgeTypes, mode, workspace])

  const handleModeChange = useCallback(
    (next: NodeKind) => {
      if (next === mode) return
      setMode(next)
    },
    [mode]
  )

  const handleFocusChange = useCallback(
    (v: string) => {
      setState((prev) => ({ ...prev, focus: v }))
    },
    [setState]
  )

  const handleDepthChange = useCallback(
    (d: number) => {
      setState((prev) => ({ ...prev, depth: d }))
    },
    [setState]
  )

  const handleDirectionChange = useCallback(
    (d: GraphDirection) => {
      setState((prev) => ({ ...prev, direction: d }))
    },
    [setState]
  )

  const handleEdgeTypeToggle = useCallback(
    (t: EdgeType) => {
      setState((prev) => {
        const has = prev.edgeTypes.includes(t)
        const next = has ? prev.edgeTypes.filter((x) => x !== t) : [...prev.edgeTypes, t]
        return { ...prev, edgeTypes: next.length > 0 ? next : prev.edgeTypes }
      })
    },
    [setState]
  )

  const handlePositionsSettled = useCallback(
    (positions: PositionMap) => {
      positionCache.write(positions)
    },
    [positionCache]
  )

  const handleDoubleClickCode = useCallback((nodeId: string) => {
    const url = `/ui/symbols?q=${encodeURIComponent(nodeId)}`
    window.location.href = url
  }, [])

  const handleDoubleClickKnowledge = useCallback((_nodeId: string) => { // eslint-disable-line @typescript-eslint/no-unused-vars
    // TODO: open DocDrawer via Zustand store (story 9.6 integration)
    // useDocDrawer store is defined in 9.6; will wire when rebased
  }, [])

  if (!workspace) {
    return (
      <div className="panel">
        <div style={{ padding: '48px', textAlign: 'center', color: 'var(--text-3)', fontSize: 13 }}>
          No workspace selected. Register a workspace with{' '}
          <span className="mono">nano-brain init</span> and reload.
        </div>
      </div>
    )
  }

  const nodes: GraphNode[] = data?.nodes ?? []
  const edges: GraphEdge[] = data?.edges ?? []
  const truncated = data?.truncated ?? false
  const frontierNodes = data?.frontier_nodes ?? []

  return (
    <div className="panel">
      <ControlBar
        mode={mode}
        focus={state.focus}
        depth={state.depth}
        direction={state.direction}
        edgeTypes={state.edgeTypes}
        onModeChange={handleModeChange}
        onFocusChange={handleFocusChange}
        onDepthChange={handleDepthChange}
        onDirectionChange={handleDirectionChange}
        onEdgeTypeToggle={handleEdgeTypeToggle}
        truncated={truncated}
        frontierCount={frontierNodes.length}
        nodeCount={nodes.length}
        edgeCount={edges.length}
      />

      <div className="graph-wrap" style={{ height: 480 }}>
        {!state.focus && (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--text-3)',
              fontSize: 13,
              pointerEvents: 'none',
            }}
          >
            {mode === 'symbol'
              ? 'Enter a symbol name above to explore the code graph'
              : 'Enter a memory document title or ID above to explore the knowledge graph'}
          </div>
        )}

        {state.focus && isPending && (
          <div
            className="skel"
            style={{ position: 'absolute', inset: 0 }}
          />
        )}

        {state.focus && isError && (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--err)',
              fontSize: 12,
            }}
          >
            {error instanceof Error ? error.message : 'Failed to load graph'}
          </div>
        )}

        {state.focus && !isPending && !isError && nodes.length > 0 && (
          <Suspense fallback={<GraphSkeleton />}>
            <SigmaGraph
              nodes={nodes}
              edges={edges}
              mode={mode}
              focusId={state.focus}
              frontierNodes={frontierNodes}
              cachedPositions={cachedPositions}
              onPositionsSettled={handlePositionsSettled}
              onDoubleClickCode={handleDoubleClickCode}
              onDoubleClickKnowledge={handleDoubleClickKnowledge}
            />
          </Suspense>
        )}

        {state.focus && !isPending && !isError && nodes.length === 0 && data && (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--text-3)',
              fontSize: 12,
            }}
          >
            No nodes found for{' '}
            <span className="mono" style={{ margin: '0 4px' }}>
              {state.focus}
            </span>
          </div>
        )}

        <div className="graph-controls" style={{ pointerEvents: 'none' }} />
        <GraphLegend mode={mode} />
      </div>
    </div>
  )
}
