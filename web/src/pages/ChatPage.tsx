import { useCallback, useEffect, useRef } from 'react'
import { useOutletContext } from 'react-router-dom'
import { useChatStream } from '../hooks/useChatStream'
import type { MessageItem } from '../types/conversation'
import StreamRenderer from '../components/StreamRenderer'
import ChatInput from '../components/ChatInput'
import { AlertCircle, MessageSquare } from 'lucide-react'

interface LayoutContext {
  convId: string | null
  create: (title?: string) => Promise<string | null>
  loadMessages: (id: string) => Promise<MessageItem[]>
  messagesLoading: boolean
}

export default function ChatPage() {
  const { convId, create, loadMessages, messagesLoading } = useOutletContext<LayoutContext>()
  const chat = useChatStream()
  const userSentRef = useRef(false)

  // 加载历史消息 — 每次 convId 变化时触发
  useEffect(() => {
    if (!convId) return

    // 不取消正在进行的流式回复，让其继续在后台完成
    // 切换回来后自动从 ES 重新加载已完成的内容

    userSentRef.current = false
    let cancelled = false

    loadMessages(convId).then((msgs) => {
      if (cancelled) return
      // 用户已经手动发了消息，不要覆盖消息状态
      if (userSentRef.current) return
      chat.clear()
      if (msgs.length > 0) {
        chat.setInitialMessages(
          msgs.map((m, i) => ({
            id: `hist_${convId}_${i}`,
            role: m.role as 'user' | 'assistant',
            content: m.content,
            createdAt: new Date().toISOString(),
          })),
        )
      }
    })

    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [convId])

  const handleSend = useCallback(
    (text: string, agent?: boolean) => {
      userSentRef.current = true
      if (!convId) {
        create('新会话').then((id) => {
          if (id) chat.sendMessage(id, text, agent)
        })
        return
      }
      chat.sendMessage(convId, text, agent)
    },
    [convId, create, chat.sendMessage],
  )

  return (
    <div className="flex flex-1 flex-col min-h-0 min-w-0">
      {chat.error && (
        <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 border-b border-red-200 dark:border-red-800 px-4 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle size={16} />
          <span>{chat.error}</span>
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-4 min-h-0">
        <div className="mx-auto max-w-3xl py-4">
          {/* 加载中 */}
          {messagesLoading && chat.messages.length === 0 && (
            <div className="flex justify-center py-12">
              <div className="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full" />
            </div>
          )}

          {/* 已选中会话但无消息（历史为空） */}
          {!messagesLoading && chat.messages.length === 0 && convId && (
            <div className="flex flex-col items-center py-16 text-gray-400">
              <MessageSquare size={48} className="mb-4 opacity-40" />
              <p className="text-lg">开始与新 AI 助手对话</p>
              <p className="text-sm mt-1">在下方输入消息开始对话</p>
            </div>
          )}

          {/* 未选中会话，欢迎页 */}
          {!messagesLoading && chat.messages.length === 0 && !convId && (
            <div className="flex flex-col items-center py-16 text-gray-400">
              <MessageSquare size={48} className="mb-4 opacity-40" />
              <p className="text-lg">开始一个新对话</p>
              <p className="text-sm mt-1">在下方输入消息开始与 AI 助手对话</p>
            </div>
          )}

          {/* 有消息时渲染 */}
          {chat.messages.length > 0 && (
            <StreamRenderer
              messages={chat.messages}
              isStreaming={chat.status === 'streaming'}
            />
          )}
        </div>
      </div>

      <ChatInput
        onSend={handleSend}
        onCancel={chat.cancel}
        disabled={chat.status === 'tool_calling'}
        streaming={chat.status === 'streaming' || chat.status === 'tool_calling'}
      />
    </div>
  )
}
