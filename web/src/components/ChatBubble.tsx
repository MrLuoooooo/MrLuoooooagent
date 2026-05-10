import type { Message } from '../types/chat'
import { Bot, User } from 'lucide-react'
import ToolCallBadge from './ToolCallBadge'

interface ChatBubbleProps {
  message: Message
  isStreaming?: boolean
}

export default function ChatBubble({ message, isStreaming }: ChatBubbleProps) {
  const isUser = message.role === 'user'

  return (
    <div className={`flex gap-3 ${isUser ? 'flex-row-reverse' : ''}`}>
      {/* 头像 */}
      <div className={`flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center ${
        isUser ? 'bg-blue-500' : 'bg-gray-400 dark:bg-gray-600'
      }`}>
        {isUser ? (
          <User size={16} className="text-white" />
        ) : (
          <Bot size={16} className="text-white" />
        )}
      </div>

      {/* 气泡内容 */}
      <div className={`max-w-[75%] ${isUser ? 'text-right' : ''}`}>
        <div
          className={`inline-block rounded-2xl px-4 py-2 text-sm leading-relaxed ${
            isUser
              ? 'bg-blue-500 text-white rounded-tr-md'
              : 'bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100 rounded-tl-md'
          }`}
        >
          <span className="whitespace-pre-wrap break-words">
            {message.content}
            {isStreaming && isUser === false && (
              <span className="inline-block w-[2px] h-4 ml-0.5 bg-current animate-blink" />
            )}
          </span>
        </div>

        {/* 工具调用状态 */}
        {message.toolCalls && message.toolCalls.length > 0 && (
          <div className="mt-2 space-y-1">
            {message.toolCalls.map((tc, i) => (
              <ToolCallBadge key={i} toolCall={tc} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
