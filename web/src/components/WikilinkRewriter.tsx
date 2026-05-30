import { useMemo } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeRaw from 'rehype-raw'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import { visit } from 'unist-util-visit'
import type { Root, Text, Parent } from 'mdast'
import { useResolveLinks } from '../hooks/useResolveLinks'
import type { Document } from '../api/types'

const WIKILINK_RE = /\[\[([^\][\n]{1,200})\]\]/g
function extractWikilinks(text: string): string[] {
  const found: string[] = []
  WIKILINK_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = WIKILINK_RE.exec(text)) !== null) {
    if (text[m.index - 1] !== '\\') {
      found.push(m[1].trim())
    }
  }
  return found
}

const sanitizeSchema = {
  ...defaultSchema,
  tagNames: [
    'p', 'a', 'code', 'pre', 'ul', 'ol', 'li',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'blockquote', 'em', 'strong', 'br', 'hr', 'span',
  ],
  attributes: {
    ...defaultSchema.attributes,
    a: [
      ...((defaultSchema.attributes?.a as string[]) ?? []),
      'className',
      'title',
      ['rel', 'noopener'],
      'dataWikilink',
    ],
    span: ['className', 'title', 'dataWikilinkAmbiguous'],
  },
}

interface ResolveMap {
  [query: string]: { matched: string[]; ambiguous: boolean } | undefined
}

function makeRemarkWikilinks(resolveMap: ResolveMap) {
  return function remarkWikilinks() {
    return function transformer(tree: Root) {
      visit(tree, 'text', (node: Text, index: number | undefined, parent: Parent | undefined) => {
        if (!parent || index == null) return

        const value = node.value
        const newNodes: (Text | { type: 'html'; value: string })[] = []
        let lastIndex = 0
        let matchFound = false

        WIKILINK_RE.lastIndex = 0
        let m: RegExpExecArray | null

        while ((m = WIKILINK_RE.exec(value)) !== null) {
          const isEscaped = m.index > 0 && value[m.index - 1] === '\\'

          if (isEscaped) {
            newNodes.push({ type: 'text', value: value.slice(lastIndex, m.index - 1) })
            newNodes.push({ type: 'text', value: m[0] })
            lastIndex = m.index + m[0].length
            matchFound = true
            continue
          }

          matchFound = true
          const target = m[1].trim()

          if (m.index > lastIndex) {
            newNodes.push({ type: 'text', value: value.slice(lastIndex, m.index) })
          }

          const resolved = resolveMap[target]

          if (!resolved || resolved.matched.length === 0) {
            newNodes.push({
              type: 'html',
              value: `<span class="wikilink-broken" title="No document with that ID or title in this workspace">${target}</span>`,
            })
          } else if (resolved.matched.length === 1) {
            const docId = resolved.matched[0]
            newNodes.push({
              type: 'html',
              value: `<a class="wikilink" data-wikilink="${docId}" href="#" rel="noopener">${target}</a>`,
            })
          } else if (resolved.ambiguous) {
            const candidates = resolved.matched.join(', ')
            newNodes.push({
              type: 'html',
              value: `<span class="wikilink-broken" data-wikilink-ambiguous="true" title="Ambiguous: ${candidates}">${target}</span>`,
            })
          } else {
            newNodes.push({
              type: 'html',
              value: `<span class="wikilink-broken" title="No document with that ID or title in this workspace">${target}</span>`,
            })
          }

          lastIndex = m.index + m[0].length
        }

        if (!matchFound) return

        if (lastIndex < value.length) {
          newNodes.push({ type: 'text', value: value.slice(lastIndex) })
        }

        parent.children.splice(
          index,
          1,
          ...(newNodes as Parameters<typeof parent.children.splice>[2][]),
        )
        return index + newNodes.length
      })
    }
  }
}

interface WikilinkRewriterProps {
  content: string
  workspace: string | null
  onOpenDoc: (doc: Document) => void
}

export function WikilinkRewriter({ content, workspace, onOpenDoc }: WikilinkRewriterProps) {
  const wikilinks = useMemo(() => extractWikilinks(content), [content])
  const results = useResolveLinks(workspace, wikilinks)

  const resolveMap = useMemo<ResolveMap>(() => {
    const map: ResolveMap = {}
    wikilinks.forEach((q, i) => {
      const r = results[i]
      if (r?.data) {
        map[q] = { matched: r.data.matched, ambiguous: r.data.ambiguous }
      }
    })
    return map
  }, [wikilinks, results])

  const remarkWikilinks = useMemo(() => makeRemarkWikilinks(resolveMap), [resolveMap])

  return (
    <div
      onClick={(e) => {
        const target = e.target as HTMLElement
        if (target.tagName === 'A' && target.dataset.wikilink) {
          e.preventDefault()
          onOpenDoc({ id: target.dataset.wikilink } as Document)
        }
      }}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkWikilinks]}
        rehypePlugins={[rehypeRaw, [rehypeSanitize, sanitizeSchema]]}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}

