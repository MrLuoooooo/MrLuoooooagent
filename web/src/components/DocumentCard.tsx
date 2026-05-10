import { FileText } from 'lucide-react'
import type { DocumentItem } from '../types/document'

interface DocumentCardProps {
  document: DocumentItem
}

export default function DocumentCard({ document }: DocumentCardProps) {
  const date = document.created_at
    ? new Date(document.created_at).toLocaleString('zh-CN')
    : '未知'

  return (
    <div className="flex items-center gap-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4">
      <div className="flex-shrink-0 rounded-lg bg-blue-50 dark:bg-blue-900/20 p-2">
        <FileText size={20} className="text-blue-500" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
          {document.title || document.document_id}
        </p>
        <p className="text-xs text-gray-400 mt-0.5">
          {document.chunk_count} 个片段 · {date}
        </p>
      </div>
    </div>
  )
}
