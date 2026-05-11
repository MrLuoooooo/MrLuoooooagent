import type { Message } from '../types/chat'
import ChatBubble from './ChatBubble'
import { MessageSquare } from 'lucide-react'

interface StreamRendererProps {
  messages: Message[]
  isStreaming: boolean
}

/**
 * 消息列表渲染器，包含消息分组、头像显示规则和间距控制。
 *
 * 头像规则：
 *   用户消息（右对齐）→ 组内最后一条显示头像
 *   助手消息（左对齐）→ 组内第一条显示头像
 *
 * 间距规则：
 *   同角色连续消息 → 2px (mt-0.5)
 *   不同角色切换 → 16px (mt-4)
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
    <div className="py-4">
      {messages.map((msg, i) => {
        const isUser = msg.role === 'user'
        const prevMsg = i > 0 ? messages[i - 1] : null
        const nextMsg = i < messages.length - 1 ? messages[i + 1] : null

        // 头像规则：
        //   用户 → 组内最后一条 (next 角色不同或没有 next)
        //   助手 → 组内第一条 (prev 角色不同或没有 prev)
        const showAvatar = isUser
          ? !nextMsg || nextMsg.role !== msg.role
          : !prevMsg || prevMsg.role !== msg.role

        // 间距：同角色连续 2px，切换角色 16px
        const isConsecutive = prevMsg && prevMsg.role === msg.role
        const spacingClass = isConsecutive ? 'mt-0.5' : 'mt-4'

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
