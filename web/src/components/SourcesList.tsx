import { ExternalLink, BookOpen, TrendingUp, Globe } from 'lucide-react'
import type { SourceRef } from '../types/chat'

interface SourcesListProps {
  sources: SourceRef[]
}

const kindMeta: Record<
  SourceRef['kind'],
  { label: string; icon: typeof Globe; cls: string }
> = {
  web: { label: '网页', icon: Globe, cls: 'text-blue-500 bg-blue-500/10 border-blue-500/30' },
  knowledge: { label: '知识库', icon: BookOpen, cls: 'text-purple-500 bg-purple-500/10 border-purple-500/30' },
  stock: { label: '行情', icon: TrendingUp, cls: 'text-emerald-500 bg-emerald-500/10 border-emerald-500/30' },
}

function hostOf(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
}

/** 参考来源区块：展示本条回复引用的数据来源（网页/知识库/行情），带链接可点击。 */
export default function SourcesList({ sources }: SourcesListProps) {
  if (!sources || sources.length === 0) return null

  return (
    <div className="mt-2 rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/60 overflow-hidden">
      <div className="px-3 py-1.5 text-xs font-medium text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700 flex items-center gap-1.5">
        <ExternalLink size={12} />
        参考来源（{sources.length}）
      </div>
      <ul className="divide-y divide-gray-200 dark:divide-gray-700/60">
        {sources.map((s, i) => {
          const meta = kindMeta[s.kind] ?? kindMeta.web
          const Icon = meta.icon
          const inner = (
            <>
              <span
                className={`flex-shrink-0 inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] border ${meta.cls}`}
              >
                <Icon size={10} />
                {meta.label}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-xs text-gray-700 dark:text-gray-200 truncate">
                  {s.title}
                </span>
                {s.url && (
                  <span className="block text-[10px] text-blue-500 dark:text-blue-400 truncate">
                    {hostOf(s.url)}
                  </span>
                )}
              </span>
              {s.url && <ExternalLink size={12} className="flex-shrink-0 text-gray-400" />}
            </>
          )
          return (
            <li key={i} className="px-3 py-1.5">
              {s.url ? (
                <a
                  href={s.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-2 hover:bg-gray-100 dark:hover:bg-gray-800 -mx-1 px-1 py-0.5 rounded transition-colors"
                >
                  {inner}
                </a>
              ) : (
                <div className="flex items-center gap-2">{inner}</div>
              )}
            </li>
          )
        })}
      </ul>
    </div>
  )
}
