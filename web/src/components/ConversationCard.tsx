import { MessageSquare, Clock } from 'lucide-react'
import type { ConversationItem } from '../types/conversation'

interface ConversationCardProps {
  conversation: ConversationItem
  active?: boolean
  onClick?: () => void
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return '刚刚'
  if (mins < 60) return `${mins}分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}天前`
  return `${Math.floor(days / 30)}个月前`
}

export default function ConversationCard({
  conversation,
  active,
  onClick,
}: ConversationCardProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full text-left rounded-lg px-3 py-2 text-sm transition-all duration-150 border ${
        active
          ? 'bg-blue-50 dark:bg-blue-950/40 border-blue-200 dark:border-blue-800/50 text-blue-700 dark:text-blue-300 shadow-sm'
          : 'border-transparent hover:bg-gray-100 dark:hover:bg-gray-800/60 text-gray-700 dark:text-gray-300 hover:border-gray-200 dark:hover:border-gray-700'
      }`}
    >
      <div className="flex items-center gap-2">
        <div className={`flex-shrink-0 w-6 h-6 rounded-md flex items-center justify-center ${
          active
            ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400'
            : 'bg-gray-100 dark:bg-gray-800 text-gray-400'
        }`}>
          <MessageSquare size={12} />
        </div>
        <span className="truncate font-medium">{conversation.title}</span>
      </div>
      <div className="flex items-center gap-2 mt-1 pl-8">
        <Clock size={10} className="text-gray-400" />
        <span className="text-[11px] text-gray-400 dark:text-gray-500">
          {timeAgo(conversation.created_at)} · {conversation.message_count} 条消息
        </span>
      </div>
    </button>
  )
}
