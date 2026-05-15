import { Send, Square, Bot } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'

interface ChatInputProps {
  onSend: (text: string, agent?: boolean) => void
  onCancel: () => void
  disabled?: boolean
  streaming?: boolean
  placeholder?: string
}

export default function ChatInput({
  onSend,
  onCancel,
  disabled,
  streaming,
  placeholder = '输入消息…',
}: ChatInputProps) {
  const [text, setText] = useState('')
  const [agentMode, setAgentMode] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // 自动调整高度
  useEffect(() => {
    const el = textareaRef.current
    if (el) {
      el.style.height = 'auto'
      el.style.height = Math.min(el.scrollHeight, 200) + 'px'
    }
  }, [text])

  const handleSend = () => {
    const trimmed = text.trim()
    if (!trimmed || disabled || streaming) return
    onSend(trimmed, agentMode)
    setText('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 p-4">
      <div className="mx-auto max-w-3xl flex items-end gap-2">
        {/* Agent 模式切换 */}
        <button
          onClick={() => setAgentMode(!agentMode)}
          disabled={disabled || streaming}
          className={`flex-shrink-0 rounded-xl px-3 py-3 text-sm font-medium transition-colors ${
            agentMode
              ? 'bg-purple-500 text-white hover:bg-purple-600'
              : 'bg-gray-100 dark:bg-gray-800 text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-700'
          } disabled:opacity-50`}
          title={agentMode ? 'Agent 模式（可调用工具）' : 'RAG 模式'}
        >
          <Bot size={16} />
        </button>

        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={agentMode ? 'Agent 模式 — AI 可搜索/查时间…' : placeholder}
          rows={1}
          disabled={disabled}
          className="flex-1 resize-none rounded-xl border border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 px-4 py-3 text-sm outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
        />

        {streaming ? (
          <button
            onClick={onCancel}
            className="flex-shrink-0 rounded-xl bg-red-500 px-4 py-3 text-white hover:bg-red-600 transition-colors"
            title="停止生成"
          >
            <Square size={16} />
          </button>
        ) : (
          <button
            onClick={handleSend}
            disabled={!text.trim() || disabled}
            className="flex-shrink-0 rounded-xl bg-blue-500 px-4 py-3 text-white hover:bg-blue-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            title="发送"
          >
            <Send size={16} />
          </button>
        )}
      </div>
    </div>
  )
}
