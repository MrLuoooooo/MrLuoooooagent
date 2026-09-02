import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Components } from 'react-markdown'

interface MarkdownContentProps {
  content: string
}

/**
 * 统一的 markdown 排版（暗色气泡内使用），StockPage 与 ChatBubble 共用。
 * 模型输出大量 markdown（**bold** / ## 标题 / 表格），此前裸文本渲染观感极差。
 * remark-gfm 支持表格、删除线、任务列表。
 */
const components: Components = {
  h1: ({ children }) => <h3 className="text-base font-semibold text-white mt-3 mb-1.5">{children}</h3>,
  h2: ({ children }) => <h3 className="text-base font-semibold text-white mt-3 mb-1.5">{children}</h3>,
  h3: ({ children }) => <h4 className="text-sm font-semibold text-white mt-2.5 mb-1">{children}</h4>,
  h4: ({ children }) => <h5 className="text-sm font-semibold text-gray-100 mt-2 mb-1">{children}</h5>,
  p: ({ children }) => <p className="leading-relaxed my-1.5">{children}</p>,
  strong: ({ children }) => <strong className="font-semibold text-white">{children}</strong>,
  em: ({ children }) => <em className="italic text-gray-100">{children}</em>,
  a: ({ children, href }) => (
    <a href={href} target="_blank" rel="noreferrer" className="text-blue-400 hover:text-blue-300 underline underline-offset-2">
      {children}
    </a>
  ),
  ul: ({ children }) => <ul className="list-disc pl-5 my-1.5 space-y-0.5">{children}</ul>,
  ol: ({ children }) => <ol className="list-decimal pl-5 my-1.5 space-y-0.5">{children}</ol>,
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="border-l-2 border-gray-600 pl-2.5 my-1.5 text-gray-400">{children}</blockquote>
  ),
  code: ({ className, children }) => {
    const isBlock = typeof className === 'string' && className.includes('language-')
    if (isBlock) {
      return <code className="block font-mono text-xs text-amber-300 bg-transparent p-0">{children}</code>
    }
    return <code className="font-mono text-xs text-amber-300 bg-gray-900 rounded px-1 py-0.5">{children}</code>
  },
  pre: ({ children }) => (
    <pre className="bg-gray-900 rounded-lg p-2.5 my-1.5 overflow-x-auto border border-gray-700">{children}</pre>
  ),
  table: ({ children }) => (
    <div className="overflow-x-auto my-1.5">
      <table className="text-xs border-collapse">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border border-gray-700 bg-gray-900 px-2 py-1 text-left font-semibold text-gray-100">{children}</th>
  ),
  td: ({ children }) => <td className="border border-gray-800 px-2 py-1 text-gray-300">{children}</td>,
  hr: () => <hr className="border-gray-700 my-2.5" />,
}

export default function MarkdownContent({ content }: MarkdownContentProps) {
  return (
    <div className="text-sm text-gray-200 break-words [&>*:first-child]:mt-0 [&>*:last-child]:mb-0">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  )
}
