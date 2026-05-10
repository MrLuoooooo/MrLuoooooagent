import type { Message } from '../types/chat'
import ChatBubble from './ChatBubble'
import { MessageSquare } from 'lucide-react'

interface StreamRendererProps {
  messages: Message[]
  isStreaming: boolean
}

/**
 * 流式消息渲染器。
 * 展示对话消息列表，流式生成时最后一条展示打字机光标。
 */
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
    <div className="space-y-6 py-4">
      {messages.map((msg, i) => (
        <ChatBubble
          key={msg.id}
          message={msg}
          isStreaming={isStreaming && i === messages.length - 1 && msg.role === 'assistant'}
        />
      ))}
    </div>
  )
}
