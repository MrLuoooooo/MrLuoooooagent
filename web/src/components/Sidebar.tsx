import { MessageSquare, Plus, Trash2, Search } from 'lucide-react'
import type { ConversationItem } from '../types/conversation'
import ConversationCard from './ConversationCard'
import { useState } from 'react'
import { deleteAllConversations } from '../api/conversation'

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
}: SidebarProps) {
  const [search, setSearch] = useState('')

  const list = Array.isArray(conversations) ? conversations : []
  const filtered = search.trim()
    ? list.filter((c) => c.title.toLowerCase().includes(search.toLowerCase()))
    : list

  return (
    <div className="flex flex-1 flex-col min-h-0 bg-white dark:bg-gray-950">
      {/* 新建会话 */}
      <div className="px-2 pt-2 pb-1.5">
        <button
          onClick={() => { onNew?.() }}
          className="w-full flex items-center justify-center gap-1.5 rounded-lg border-2 border-dashed border-gray-200 dark:border-gray-700 px-3 py-2 text-sm font-medium text-gray-500 dark:text-gray-400 hover:border-blue-400 hover:text-blue-500 dark:hover:border-blue-600 dark:hover:text-blue-400 transition-all duration-150 hover:bg-blue-50/50 dark:hover:bg-blue-900/10"
        >
          <Plus size={16} />
          <span>新建会话</span>
        </button>
      </div>

      {/* 搜索框 */}
      <div className="px-2 pb-1.5">
        <div className="relative">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-400" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索会话…"
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 pl-7 pr-2.5 py-1.5 text-xs outline-none focus:border-blue-400 dark:focus:border-blue-600 transition-colors placeholder:text-gray-400"
          />
        </div>
      </div>

      {/* 会话列表 */}
      <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-0.5 scrollbar-thin">
        {loading && (
          <div className="flex flex-col items-center py-8 text-gray-400">
            <div className="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full mb-2" />
            <span className="text-xs">加载中…</span>
          </div>
        )}
        {!loading && filtered.length === 0 && (
          <div className="text-sm text-gray-400 text-center py-8 space-y-2">
            <MessageSquare size={28} className="mx-auto opacity-30" />
            <p>{search ? '未找到匹配会话' : '暂无会话'}</p>
            {!search && <p className="text-xs">点击上方按钮开始新对话</p>}
          </div>
        )}
        {filtered.map((conv) => (
          <div key={conv.conversation_id} className="group relative">
            <ConversationCard
              conversation={conv}
              active={conv.conversation_id === currentId}
              onClick={() => { onSelect?.(conv.conversation_id) }}
            />
            {onDelete && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  onDelete(conv.conversation_id)
                }}
                className="absolute right-1.5 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 p-1 rounded-md hover:bg-red-100 dark:hover:bg-red-900/30 text-gray-400 hover:text-red-500 transition-all duration-150"
                title="删除会话"
              >
                <Trash2 size={13} />
              </button>
            )}
          </div>
        ))}
      </div>

      {!loading && list.length > 0 && (
        <div className="flex-shrink-0 px-3 py-2 border-t border-gray-100 dark:border-gray-800 space-y-1.5">
          <p className="text-[11px] text-gray-400">
            共 {list.length} 个会话
          </p>
          <button
            onClick={() => {
              if (confirm('确定清空所有会话？此操作不可恢复。')) {
                deleteAllConversations().then(() => window.location.reload())
              }
            }}
            className="w-full flex items-center justify-center gap-1 rounded-md border border-red-200 dark:border-red-900/50 px-2 py-1.5 text-[11px] text-red-500 hover:bg-red-50 dark:hover:bg-red-900/10 transition-colors"
          >
            <Trash2 size={12} />
            清空所有会话
          </button>
        </div>
      )}
    </div>
  )
}
