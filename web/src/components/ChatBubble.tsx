import type { Message } from '../types/chat'
import { Bot, User } from 'lucide-react'
import ToolCallBadge from './ToolCallBadge'
import SourcesList from './SourcesList'
import MarkdownContent from './MarkdownContent'

interface ChatBubbleProps {
  message: Message
  showAvatar: boolean
  isStreaming?: boolean
}

export default function ChatBubble({ message, showAvatar, isStreaming }: ChatBubbleProps) {
  const isUser = message.role === 'user'

  const avatarNode = (
    <div
      className={`flex-shrink-0 w-7 h-7 rounded-full flex items-center justify-center text-white shadow-sm ${
        isUser
          ? 'bg-gradient-to-br from-blue-500 to-blue-600'
          : 'bg-gradient-to-br from-gray-400 to-gray-500 dark:from-gray-500 dark:to-gray-600'
      }`}
    >
      {isUser ? <User size={14} /> : <Bot size={14} />}
    </div>
  )

  const spacerNode = <div className="flex-shrink-0 w-7 h-7" />

  const bubbleContent = (
    <div className="min-w-0">
      <div
        className={`inline-block px-4 py-2.5 text-sm leading-relaxed ${
          isUser
            ? 'bg-gradient-to-br from-blue-500 to-blue-600 text-white rounded-2xl rounded-br-md shadow-sm shadow-blue-200/50 dark:shadow-blue-900/30'
            : 'bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100 rounded-2xl rounded-bl-md border border-gray-200/50 dark:border-gray-700/50'
        }`}
      >
        {isUser ? (
          <span className="whitespace-pre-wrap break-words">{message.content}</span>
        ) : (
          <MarkdownContent content={message.content} />
        )}
        {isStreaming && (
          <span className="inline-block w-[2px] h-4 ml-0.5 bg-blue-500 dark:bg-blue-400 animate-blink rounded-sm align-middle" />
        )}
      </div>

      {message.toolCalls && message.toolCalls.length > 0 && (
        <div className="mt-2 space-y-1">
          {message.toolCalls.map((tc, i) => (
            <ToolCallBadge key={i} toolCall={tc} />
          ))}
        </div>
      )}

      {!isUser && message.sources && message.sources.length > 0 && (
        <SourcesList sources={message.sources} />
      )}
    </div>
  )

  return (
    <div className={`flex items-end gap-2 ${isUser ? 'ml-auto w-fit max-w-[85%]' : 'max-w-[85%]'}`}>
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
