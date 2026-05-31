import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'

const sanitizeSchema = {
  ...defaultSchema,
  tagNames: ['p', 'a', 'code', 'pre', 'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'blockquote', 'em', 'strong', 'br', 'hr'],
  attributes: {
    ...defaultSchema.attributes,
    a: ['href', 'title', ['rel', 'noopener']],
  },
}

interface SafeMarkdownProps {
  content: string
}

export function SafeMarkdown({ content }: SafeMarkdownProps) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[[rehypeSanitize, sanitizeSchema]]}
    >
      {content}
    </ReactMarkdown>
  )
}
