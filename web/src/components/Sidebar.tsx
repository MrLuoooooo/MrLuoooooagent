import { MessageSquare, Plus, Trash2 } from 'lucide-react'
import type { ConversationItem } from '../types/conversation'
import ConversationCard from './ConversationCard'

interface SidebarProps {
  conversations?: ConversationItem[]
  currentId?: string | null
  onSelect?: (id: string) => void
  onNew?: () => void
  onDelete?: (id: string) => void
  loading?: boolean
  onClose?: () => void
}

export default function Sidebar({
  conversations = [],
  currentId,
  onSelect,
  onNew,
  onDelete,
  loading,
  onClose,
}: SidebarProps) {
  return (
    <div className="flex h-full flex-col bg-gray-50 dark:bg-gray-950 border-r border-gray-200 dark:border-gray-800">
      {/* 顶栏 */}
      <div className="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-gray-800">
        <h1 className="text-lg font-bold flex items-center gap-2">
          <MessageSquare size={20} />
          GoAgent
        </h1>
        <button
          onClick={() => { onNew?.(); onClose?.() }}
          className="rounded-lg p-2 hover:bg-gray-200 dark:hover:bg-gray-800"
          title="新建会话"
        >
          <Plus size={18} />
        </button>
      </div>

      {/* 会话列表 */}
      <div className="flex-1 overflow-y-auto p-2 space-y-1">
        {loading && (
          <p className="text-sm text-gray-400 text-center py-4">加载中…</p>
        )}
        {!loading && conversations.length === 0 && (
          <div className="text-sm text-gray-400 text-center py-8 space-y-2">
            <MessageSquare size={32} className="mx-auto opacity-40" />
            <p>暂无会话</p>
            <p className="text-xs">点击 + 开始新对话</p>
          </div>
        )}
        {conversations.map((conv) => (
          <div key={conv.conversation_id} className="group relative">
            <ConversationCard
              conversation={conv}
              active={conv.conversation_id === currentId}
              onClick={() => { onSelect?.(conv.conversation_id); onClose?.() }}
            />
            {onDelete && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  onDelete(conv.conversation_id)
                }}
                className="absolute right-2 top-1/2 -translate-y-1/2 hidden group-hover:block p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-400"
              >
                <Trash2 size={14} />
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
