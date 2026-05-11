import type { Message } from '../types/chat'
import { Bot, User } from 'lucide-react'
import ToolCallBadge from './ToolCallBadge'

interface ChatBubbleProps {
  message: Message
  showAvatar: boolean
  isStreaming?: boolean
}

/**
 * 消息气泡组件。
 *
 * 布局：flex 行，avatar 和 bubble 并排，gap-1.5 紧贴。
 *   用户消息 → ml-auto 推右，[bubble] [avatar]
 *   助手消息 → 左对齐，[avatar] [bubble]
 * 当 showAvatar=false 时用一个同宽的空 div 占位保持对齐。
 */
export default function ChatBubble({ message, showAvatar, isStreaming }: ChatBubbleProps) {
  const isUser = message.role === 'user'

  const avatarNode = (
    <div
      className="flex-shrink-0 w-7 h-7 rounded-full flex items-center justify-center text-white"
      style={{ backgroundColor: isUser ? '#3b82f6' : '#9ca3af' }}
    >
      {isUser ? <User size={14} /> : <Bot size={14} />}
    </div>
  )

  const spacerNode = <div className="flex-shrink-0 w-7 h-7" />

  const bubbleContent = (
    <div>
      <div
        className={`inline-block px-3.5 py-2 text-sm leading-relaxed ${
          isUser
            ? 'bg-blue-500 text-white rounded-2xl rounded-br-md'
            : 'bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100 rounded-2xl rounded-bl-md'
        }`}
      >
        <span className="whitespace-pre-wrap break-words">
          {message.content}
          {isStreaming && (
            <span className="inline-block w-[2px] h-4 ml-0.5 bg-current animate-blink" />
          )}
        </span>
      </div>

      {message.toolCalls && message.toolCalls.length > 0 && (
        <div className="mt-2 space-y-1">
          {message.toolCalls.map((tc, i) => (
            <ToolCallBadge key={i} toolCall={tc} />
          ))}
        </div>
      )}
    </div>
  )

  return (
    <div className={`flex items-end gap-1.5 ${isUser ? 'ml-auto w-fit' : ''}`}>
      {isUser ? (
        <>
          {bubbleContent}
          {showAvatar ? avatarNode : spacerNode}
        </>
      ) : (
        <>
          {showAvatar ? avatarNode : spacerNode}
          {bubbleContent}
        </>
      )}
    </div>
  )
}
