import { useEffect, useRef, useCallback } from 'react'
import Graph from 'graphology'
import Sigma from 'sigma'
import FA2Worker from 'graphology-layout-forceatlas2/worker'
import type { GraphNode, GraphEdge, NodeKind } from '../../api/types'
import type { PositionMap } from './usePositionCache'

const CODE_EDGE_COLORS: Record<string, string> = {
  calls: '#0070F3',
  imports: '#F59E0B',
  contains: '#10B981',
}

const KNOWLEDGE_COLLECTION_COLORS: Record<string, string> = {
  memory: '#0070F3',
  'session-summary:opencode': '#10B981',
  'session-summary:claudecode': '#F59E0B',
}

function nodeColor(node: GraphNode, mode: NodeKind): string {
  if (mode === 'symbol') {
    return '#4B5563'
  }
  return KNOWLEDGE_COLLECTION_COLORS[node.collection ?? ''] ?? '#6B7280'
}

function edgeColor(edge: GraphEdge, mode: NodeKind): string {
  if (mode === 'symbol') {
    return CODE_EDGE_COLORS[edge.edge_type] ?? '#4B5563'
  }
  return '#0070F3'
}

function fmtAge(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return s + 's ago'
  const m = Math.floor(s / 60)
  if (m < 60) return m + 'm ago'
  const h = Math.floor(m / 60)
  if (h < 24) return h + 'h ago'
  return Math.floor(h / 24) + 'd ago'
}

export interface SigmaGraphProps {
  nodes: GraphNode[]
  edges: GraphEdge[]
  mode: NodeKind
  focusId: string
  frontierNodes: string[]
  cachedPositions: PositionMap | null
  onPositionsSettled: (positions: PositionMap) => void
  onDoubleClickCode: (nodeId: string) => void
  onDoubleClickKnowledge: (nodeId: string) => void
}

export function SigmaGraph({
  nodes,
  edges,
  mode,
  focusId,
  frontierNodes,
  cachedPositions,
  onPositionsSettled,
  onDoubleClickCode,
  onDoubleClickKnowledge,
}: SigmaGraphProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const sigmaRef = useRef<Sigma | null>(null)
  const workerRef = useRef<FA2Worker | null>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)
  const settledRef = useRef(false)

  const buildGraph = useCallback(() => {
    const g = new Graph({ multi: false, type: 'directed' })

    const useCached = !!cachedPositions

    nodes.forEach((n) => {
      const cached = cachedPositions?.[n.id]
      const x = cached ? cached[0] : Math.random() * 10
      const y = cached ? cached[1] : Math.random() * 10
      const isFocus = n.id === focusId
      const isFrontier = frontierNodes.includes(n.id)
      const color = isFocus ? '#0070F3' : nodeColor(n, mode)
      g.addNode(n.id, {
        x,
        y,
        size: isFocus ? 12 : isFrontier ? 9 : 6,
        color,
        label: mode === 'symbol' ? n.id : (n.title?.slice(0, 40) ?? n.id),
        borderColor: isFrontier ? '#F59E0B' : undefined,
        type: isFrontier ? 'border' : 'circle',
      })
    })

    edges.forEach((e, i) => {
      if (!g.hasNode(e.source) || !g.hasNode(e.target)) return
      const eid = `${e.source}->${e.target}-${i}`
      if (g.hasEdge(eid)) return
      g.addEdge(e.source, e.target, {
        color: edgeColor(e, mode),
        size: 1.2,
        type: 'arrow',
      })
    })

    return { g, useCached }
  }, [nodes, edges, mode, focusId, frontierNodes, cachedPositions])

  useEffect(() => {
    const container = containerRef.current
    if (!container || nodes.length === 0) return

    const { g, useCached } = buildGraph()
    settledRef.current = false

    const sigma = new Sigma(g, container, {
      renderEdgeLabels: false,
      defaultEdgeType: 'arrow',
      labelFont: '"JetBrains Mono", monospace',
      labelSize: 11,
      labelColor: { color: 'rgba(255,255,255,0.85)' },
    })
    sigmaRef.current = sigma

    if (!useCached) {
      const worker = new FA2Worker(g, {
        settings: {
          gravity: 1,
          scalingRatio: 2,
          slowDown: 5,
          barnesHutOptimize: nodes.length > 200,
        },
      })
      workerRef.current = worker
      worker.start()

      const settle = setTimeout(() => {
        worker.stop()
        settledRef.current = true
        const positions: PositionMap = {}
        g.forEachNode((id, attrs) => {
          positions[id] = [attrs.x as number, attrs.y as number]
        })
        onPositionsSettled(positions)
      }, 2000)

      return () => {
        clearTimeout(settle)
        worker.kill()
        workerRef.current = null
        sigma.kill()
        sigmaRef.current = null
      }
    } else {
      settledRef.current = true
      return () => {
        sigma.kill()
        sigmaRef.current = null
      }
    }
  }, [buildGraph, nodes.length, onPositionsSettled])

  useEffect(() => {
    const sigma = sigmaRef.current
    const tooltip = tooltipRef.current
    if (!sigma || !tooltip) return

    const onEnterNode = ({ node }: { node: string }) => {
      const g = sigma.getGraph()
      const attrs = g.getNodeAttributes(node)
      const nodeData = nodes.find((n) => n.id === node)
      if (!nodeData) return

      let html = ''
      if (mode === 'symbol') {
        html = `<strong>${nodeData.id}</strong><br/><span style="color:var(--text-3)">${nodeData.source_file ?? ''}</span>`
      } else {
        const age = nodeData.updated_at ? fmtAge(nodeData.updated_at) : ''
        html = `<strong>${nodeData.title ?? nodeData.id}</strong><br/><span style="color:var(--text-3)">${nodeData.collection ?? ''} · ${age}</span>`
      }
      tooltip.innerHTML = html
      tooltip.style.display = 'block'
      const pos = sigma.graphToViewport({ x: attrs.x as number, y: attrs.y as number })
      tooltip.style.left = pos.x + 16 + 'px'
      tooltip.style.top = pos.y - 10 + 'px'
    }

    const onLeaveNode = () => {
      tooltip.style.display = 'none'
    }

    sigma.on('enterNode', onEnterNode)
    sigma.on('leaveNode', onLeaveNode)

    return () => {
      sigma.off('enterNode', onEnterNode)
      sigma.off('leaveNode', onLeaveNode)
    }
  }, [nodes, mode])

  useEffect(() => {
    const sigma = sigmaRef.current
    if (!sigma) return

    const onDoubleClick = ({ node }: { node: string }) => {
      if (!node) return
      if (mode === 'symbol') {
        onDoubleClickCode(node)
      } else {
        onDoubleClickKnowledge(node)
      }
    }

    sigma.on('doubleClickNode', onDoubleClick)
    return () => {
      sigma.off('doubleClickNode', onDoubleClick)
    }
  }, [mode, onDoubleClickCode, onDoubleClickKnowledge])

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%' }}>
      <div ref={containerRef} style={{ width: '100%', height: '100%' }} />
      <div
        ref={tooltipRef}
        style={{
          display: 'none',
          position: 'absolute',
          pointerEvents: 'none',
          background: 'var(--bg-1)',
          border: '1px solid var(--border)',
          padding: '6px 10px',
          fontSize: 11,
          fontFamily: 'inherit',
          lineHeight: 1.5,
          maxWidth: 280,
          zIndex: 10,
          color: 'var(--text-1)',
        }}
      />
    </div>
  )
}
