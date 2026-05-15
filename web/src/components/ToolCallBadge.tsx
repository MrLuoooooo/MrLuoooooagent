import { Loader2, CheckCircle2, XCircle } from 'lucide-react'
import type { ToolCall } from '../types/chat'

interface ToolCallBadgeProps {
  toolCall: ToolCall
}

export default function ToolCallBadge({ toolCall }: ToolCallBadgeProps) {
  const statusIcon = () => {
    switch (toolCall.status) {
      case 'pending':
      case 'running':
        return <Loader2 size={14} className="animate-spin" />
      case 'done':
        return <CheckCircle2 size={14} className="text-green-500" />
      case 'error':
        return <XCircle size={14} className="text-red-500" />
    }
  }

  const statusText = () => {
    switch (toolCall.status) {
      case 'pending':
        return '等待中…'
      case 'running':
        return '正在处理…'
      case 'done':
        return '完成'
      case 'error':
        return '失败'
    }
  }

  return (
    <div className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 dark:bg-gray-800 px-3 py-1 text-xs text-gray-600 dark:text-gray-400">
      {statusIcon()}
      <span>{toolCall.name}</span>
      <span className="text-gray-400">·</span>
      <span>{statusText()}</span>
    </div>
  )
}
