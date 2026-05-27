import type { Message } from '../types/chat'
import ChatBubble from './ChatBubble'
import { MessageSquare } from 'lucide-react'

interface StreamRendererProps {
  messages: Message[]
  isStreaming: boolean
}

export default function StreamRenderer({ messages, isStreaming }: StreamRendererProps) {
  if (messages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-gray-400 dark:text-gray-600">
        <MessageSquare size={48} className="mb-4 opacity-30" />
        <p className="text-lg">开始一段新对话</p>
        <p className="text-sm">输入问题，AI 将为你解答</p>
      </div>
    )
  }

  return (
    <div className="py-4 space-y-3">
      {messages.map((msg, i) => {
        const isUser = msg.role === 'user'
        const prevMsg = i > 0 ? messages[i - 1] : null
        const nextMsg = i < messages.length - 1 ? messages[i + 1] : null

        const showAvatar = isUser
          ? !nextMsg || nextMsg.role !== msg.role
          : !prevMsg || prevMsg.role !== msg.role

        const isConsecutive = prevMsg && prevMsg.role === msg.role
        const spacingClass = isConsecutive ? 'mt-0.5' : 'mt-5'

        return (
          <div key={msg.id} className={spacingClass}>
            <ChatBubble
              message={msg}
              showAvatar={showAvatar}
              isStreaming={isStreaming && i === messages.length - 1 && msg.role === 'assistant'}
            />
          </div>
        )
      })}
    </div>
  )
}
