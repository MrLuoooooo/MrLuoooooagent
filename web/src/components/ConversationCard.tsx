import { MessageSquare } from 'lucide-react'
import type { ConversationItem } from '../types/conversation'

interface ConversationCardProps {
  conversation: ConversationItem
  active?: boolean
  onClick?: () => void
}

export default function ConversationCard({
  conversation,
  active,
  onClick,
}: ConversationCardProps) {
  return (
    <button
      onClick={onClick}
      className={`w-full text-left rounded-lg px-3 py-2 text-sm transition-colors ${
        active
          ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
          : 'hover:bg-gray-200 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
      }`}
    >
      <div className="flex items-center gap-2">
        <MessageSquare size={14} className="flex-shrink-0" />
        <span className="truncate">{conversation.title}</span>
      </div>
      <p className="mt-0.5 text-xs text-gray-400 pl-6">
        {conversation.message_count} 条消息
      </p>
    </button>
  )
}
