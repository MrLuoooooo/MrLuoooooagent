import { useEffect, useState, useCallback } from 'react'
import { fetchPendingApprovals, fetchAllApprovals, decideApproval, type ApprovalItem } from '../api/approval'
import { Shield, Check, X, Clock, RefreshCw, ChevronDown, ChevronUp } from 'lucide-react'

function riskBadge(level: string) {
  switch (level) {
    case '高': return 'bg-red-100 dark:bg-red-900/30 text-red-600'
    case '中': return 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-600'
    case '低': return 'bg-green-100 dark:bg-green-900/30 text-green-600'
    default: return 'bg-gray-100 text-gray-600'
  }
}

function statusIcon(status: string) {
  switch (status) {
    case 'pending': return <Clock size={14} className="text-yellow-500" />
    case 'accepted': return <Check size={14} className="text-green-500" />
    case 'rejected': return <X size={14} className="text-red-500" />
    default: return null
  }
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return '刚刚'
  if (mins < 60) return `${mins} 分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours} 小时前`
  return `${Math.floor(hours / 24)} 天前`
}

export default function ApprovalPage() {
  const [items, setItems] = useState<ApprovalItem[]>([])
  const [loading, setLoading] = useState(true)
  const [tab, setTab] = useState<'pending' | 'all'>('pending')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [deciding, setDeciding] = useState<Set<string>>(new Set())

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = tab === 'pending' ? await fetchPendingApprovals() : await fetchAllApprovals()
      setItems(data)
    } catch { /* silent */ }
    setLoading(false)
  }, [tab])

  useEffect(() => { load() }, [load])

  const handleDecide = useCallback(async (id: string, accept: boolean) => {
    setDeciding(prev => new Set(prev).add(id))
    try {
      await decideApproval(id, accept)
      setItems(prev =>
        prev.map(it => (it.id === id ? { ...it, status: accept ? 'accepted' as const : 'rejected' as const } : it)),
      )
    } catch { /* ignore */ }
    setDeciding(prev => { const s = new Set(prev); s.delete(id); return s })
  }, [])

  const toggleExpand = useCallback((id: string) => {
    setExpanded(prev => { const s = new Set(prev); s.has(id) ? s.delete(id) : s.add(id); return s })
  }, [])

  const pendingCount = items.filter(i => i.status === 'pending').length

  return (
    <div className="flex flex-1 flex-col min-h-0 min-w-0">
      <div className="flex-1 overflow-y-auto px-4 min-h-0">
        <div className="mx-auto max-w-3xl py-4 space-y-4">
          {/* Header */}
          <div className="flex items-center justify-between">
            <h2 className="font-semibold flex items-center gap-2">
              <Shield size={18} /> 审批中心
            </h2>
            <div className="flex gap-2">
              <button
                onClick={() => { setTab('pending'); setItems([]) }}
                className={`text-xs px-3 py-1 rounded-full ${tab === 'pending' ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' : 'text-gray-500'}`}
              >
                待审批 {pendingCount > 0 && `(${pendingCount})`}
              </button>
              <button
                onClick={() => { setTab('all'); setItems([]) }}
                className={`text-xs px-3 py-1 rounded-full ${tab === 'all' ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' : 'text-gray-500'}`}
              >
                全部
              </button>
              <button onClick={load} disabled={loading} className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700">
                <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
              </button>
            </div>
          </div>

          {/* Items */}
          {loading && items.length === 0 && (
            <div className="flex justify-center py-12">
              <div className="animate-spin h-8 w-8 border-4 border-purple-500 border-t-transparent rounded-full" />
            </div>
          )}
          {!loading && items.length === 0 && (
            <div className="text-center text-gray-400 py-12">
              <Shield size={32} className="mx-auto mb-2 opacity-40" />
              <p>{tab === 'pending' ? '暂无待审批项' : '暂无记录'}</p>
            </div>
          )}

          {items.map(item => (
            <div
              key={item.id}
              className={`bg-white dark:bg-gray-900 rounded-lg border px-4 py-3 space-y-2 ${
                item.status === 'pending' ? 'border-yellow-300 dark:border-yellow-700' :
                item.status === 'accepted' ? 'border-green-200 dark:border-green-800' :
                'border-gray-200 dark:border-gray-700 opacity-60'
              }`}
            >
              <div className="flex items-center gap-2">
                {statusIcon(item.status)}
                <span className="text-sm font-medium">{item.task_name}</span>
                <span className={`text-xs px-1.5 py-0.5 rounded ${riskBadge(item.risk_level)}`}>
                  {item.risk_level}风险
                </span>
                <span className="ml-auto text-xs text-gray-400">{timeAgo(item.created_at)}</span>
              </div>

              <div className="text-sm text-gray-600 dark:text-gray-300">
                <strong>{item.action_type}</strong> — {item.reason}
              </div>

              {/* Expandable output preview */}
              <button onClick={() => toggleExpand(item.id)} className="flex items-center gap-1 text-xs text-purple-500 hover:text-purple-600">
                {expanded.has(item.id) ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
                查看完整输出
              </button>
              {expanded.has(item.id) && (
                <pre className="text-xs whitespace-pre-wrap bg-gray-50 dark:bg-gray-800 rounded p-2 max-h-60 overflow-y-auto">
                  {item.full_output}
                </pre>
              )}

              {item.status === 'pending' && (
                <div className="flex gap-2 pt-1">
                  <button
                    onClick={() => handleDecide(item.id, true)}
                    disabled={deciding.has(item.id)}
                    className="flex items-center gap-1 rounded bg-green-500 px-3 py-1 text-xs text-white hover:bg-green-600 disabled:opacity-50"
                  >
                    <Check size={12} /> 批准
                  </button>
                  <button
                    onClick={() => handleDecide(item.id, false)}
                    disabled={deciding.has(item.id)}
                    className="flex items-center gap-1 rounded bg-red-500 px-3 py-1 text-xs text-white hover:bg-red-600 disabled:opacity-50"
                  >
                    <X size={12} /> 拒绝
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
