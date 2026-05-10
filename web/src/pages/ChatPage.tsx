import { useCallback, useEffect, useRef } from 'react'
import { useOutletContext } from 'react-router-dom'
import { useChatStream } from '../hooks/useChatStream'
import StreamRenderer from '../components/StreamRenderer'
import ChatInput from '../components/ChatInput'
import { AlertCircle } from 'lucide-react'

interface LayoutContext {
  convId: string | null
  create: (title?: string) => Promise<string | null>
}

export default function ChatPage() {
  const { convId, create } = useOutletContext<LayoutContext>()
  const chat = useChatStream()
  const prevConvId = useRef<string | null>(null)

  // 切换会话时清空消息
  useEffect(() => {
    if (convId && convId !== prevConvId.current) {
      chat.clear()
      prevConvId.current = convId
    }
  }, [convId, chat])

  const handleSend = useCallback(
    (text: string) => {
      if (!convId) {
        create('新会话').then((id) => {
          if (id) chat.sendMessage(id, text)
        })
        return
      }
      chat.sendMessage(convId, text)
    },
    [convId, create, chat],
  )

  return (
    <div className="flex flex-1 flex-col min-w-0">
      {chat.error && (
        <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 border-b border-red-200 dark:border-red-800 px-4 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle size={16} />
          <span>{chat.error}</span>
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-4">
        <div className="mx-auto max-w-3xl py-4">
          <StreamRenderer
            messages={chat.messages}
            isStreaming={chat.status === 'streaming'}
          />
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
