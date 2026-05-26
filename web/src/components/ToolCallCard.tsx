import { Loader2, CheckCircle2, XCircle, Wrench, ChevronDown, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import type { ToolCall } from '../types/chat'

interface ToolCallCardProps {
  toolCall: ToolCall
}

export default function ToolCallCard({ toolCall }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false)

  const statusIcon = () => {
    switch (toolCall.status) {
      case 'pending':
      case 'running':
        return <Loader2 size={14} className="animate-spin text-blue-500" />
      case 'done':
        return <CheckCircle2 size={14} className="text-green-500" />
      case 'error':
        return <XCircle size={14} className="text-red-500" />
    }
  }

  const statusColor = () => {
    switch (toolCall.status) {
      case 'running': return 'border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/30'
      case 'done': return 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/30'
      case 'error': return 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/30'
      default: return 'border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50'
    }
  }

  const hasDetails = toolCall.args || toolCall.output

  return (
    <div className={`my-1.5 border rounded-lg px-3 py-2 text-xs ${statusColor()}`}>
      <div
        className="flex items-center gap-2 cursor-pointer select-none"
        onClick={() => hasDetails && setExpanded(!expanded)}
      >
        <Wrench size={14} className="text-gray-400 shrink-0" />
        <span className="font-medium text-gray-700 dark:text-gray-300">{toolCall.name}</span>
        {statusIcon()}
        <span className="text-gray-400 ml-auto text-[11px]">
          {toolCall.status === 'running' ? '执行中' : toolCall.status === 'done' ? '完成' : ''}
        </span>
        {hasDetails && (
          <span className="text-gray-400">
            {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </span>
        )}
      </div>

      {expanded && hasDetails && (
        <div className="mt-2 pt-2 border-t border-gray-200 dark:border-gray-600 space-y-1.5">
          {toolCall.args && (
            <div>
              <span className="text-gray-400">参数: </span>
              <code className="text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded text-[11px] break-all">
                {toolCall.args.length > 200 ? toolCall.args.slice(0, 200) + '...' : toolCall.args}
              </code>
            </div>
          )}
          {toolCall.output && (
            <div>
              <span className="text-gray-400">结果: </span>
              <code className="text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded text-[11px] break-all max-h-32 overflow-y-auto block">
                {toolCall.output.length > 500 ? toolCall.output.slice(0, 500) + '...' : toolCall.output}
              </code>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
