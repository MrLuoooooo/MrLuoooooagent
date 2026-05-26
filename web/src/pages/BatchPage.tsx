import { useState, useCallback, useRef, useEffect, useMemo } from 'react'
import { batchStream, type BatchProgress, type BatchTask } from '../api/batch'
import { Layers, Play, Square, AlertCircle, Check, X, Loader2 } from 'lucide-react'

interface TaskState {
  id: string
  prompt: string
  status: 'pending' | 'running' | 'done' | 'error'
  result?: string
  error?: string
}

export default function BatchPage() {
  const [input, setInput] = useState('')
  const [agent, setAgent] = useState(true)
  const [running, setRunning] = useState(false)
  const [tasks, setTasks] = useState<TaskState[]>([])
  const [summary, setSummary] = useState('')
  const [error, setError] = useState('')
  const abortRef = useRef<AbortController | null>(null)

  const parseTasks = useCallback((text: string): BatchTask[] => {
    const lines = text.split('\n')
    const tasks: BatchTask[] = []
    let current = ''

    for (const line of lines) {
      const trimmed = line.trim()
      if (trimmed === '') {
        if (current.trim()) {
          tasks.push({ id: `task_${tasks.length + 1}`, prompt: current.trim() })
          current = ''
        }
      } else if (/^\d+[.)、]\s*/.test(trimmed)) {
        if (current.trim()) {
          tasks.push({ id: `task_${tasks.length + 1}`, prompt: current.trim() })
        }
        current = trimmed.replace(/^\d+[.)、]\s*/, '').trim()
      } else {
        if (current) current += '\n'
        current += trimmed
      }
    }
    if (current.trim()) {
      tasks.push({ id: `task_${tasks.length + 1}`, prompt: current.trim() })
    }
    return tasks
  }, [])

  const taskCount = useMemo(() => parseTasks(input).length, [input, parseTasks])

  const handleRun = useCallback(() => {
    const taskList = parseTasks(input)
    if (taskList.length === 0) return
    if (taskList.length > 10) { setError('单次最多 10 个任务'); return }

    setError('')
    setSummary('')
    setRunning(true)
    setTasks(taskList.map(t => ({ ...t, status: 'pending' as const })))

    const abrt = batchStream(
      taskList,
      agent,
      (evt: BatchProgress) => {
        setTasks(prev => {
          const next = [...prev]
          const idx = next.findIndex(t => t.id === evt.task_id)
          switch (evt.type) {
            case 'task_start':
              if (idx >= 0) next[idx] = { ...next[idx], status: 'running' }
              return next
            case 'task_done':
              if (idx >= 0) next[idx] = { ...next[idx], status: 'done', result: evt.result }
              return next
            case 'task_error':
              if (idx >= 0) next[idx] = { ...next[idx], status: 'error', error: evt.error }
              return next
            case 'summary':
              setSummary(evt.result || '')
              return next
            case 'done':
              // no state change needed
              return next
            default:
              return prev
          }
        })
      },
      (err: Error) => setError(err.message),
      () => setRunning(false),
    )
    abortRef.current = abrt
  }, [input, agent, parseTasks])

  const handleCancel = useCallback(() => {
    abortRef.current?.abort()
    setRunning(false)
    setTasks(prev =>
      prev.map(t => (t.status === 'running' ? { ...t, status: 'error', error: '已取消' } : t)),
    )
  }, [])

  // Cleanup on unmount
  useEffect(() => {
    return () => abortRef.current?.abort()
  }, [])

  return (
    <div className="flex flex-1 flex-col min-h-0 min-w-0">
      {error && (
        <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 border-b border-red-200 dark:border-red-800 px-4 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle size={16} /><span>{error}</span>
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-4 min-h-0">
        <div className="mx-auto max-w-3xl py-4 space-y-4">
          {/* 输入区 */}
          <div className="bg-white dark:bg-gray-900 rounded-xl border border-gray-200 dark:border-gray-700 p-4 space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="font-semibold flex items-center gap-2">
                <Layers size={18} /> 批量任务
              </h2>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={agent}
                  onChange={e => setAgent(e.target.checked)}
                  className="rounded border-gray-300 text-purple-600 focus:ring-purple-500"
                  disabled={running}
                />
                <span className="text-purple-600 dark:text-purple-400 font-medium">Agent 模式</span>
              </label>
            </div>

            <p className="text-xs text-gray-400">
              每行一个 prompt，空行分隔不同任务（最多 10 个）。支持数字序号：1. xxx
            </p>

            <textarea
              value={input}
              onChange={e => setInput(e.target.value)}
              disabled={running}
              rows={6}
              className="w-full rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm resize-y focus:outline-none focus:ring-2 focus:ring-purple-400 disabled:opacity-50"
              placeholder={'1. 列出 /D/goagentpro 下的所有 Go 文件\n2. 检查 Makefile 中的构建命令\n3. 读取 go.mod 并分析依赖'}
            />

            <div className="flex gap-2">
              {!running ? (
                <button
                  onClick={handleRun}
                  disabled={!input.trim()}
                  className="flex items-center gap-1 rounded-lg bg-purple-600 px-4 py-2 text-sm text-white hover:bg-purple-700 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <Play size={14} /> 批量执行
                </button>
              ) : (
                <button
                  onClick={handleCancel}
                  className="flex items-center gap-1 rounded-lg bg-red-500 px-4 py-2 text-sm text-white hover:bg-red-600"
                >
                  <Square size={14} /> 取消
                </button>
              )}
              {!running && tasks.length > 0 && taskCount > 0 && (
                <span className="self-center text-xs text-gray-400">
                  将执行 {taskCount} 个任务
                </span>
              )}
            </div>
          </div>

          {/* 任务进度卡片 */}
          {tasks.map((task) => (
            <div
              key={task.id}
              className={`bg-white dark:bg-gray-900 rounded-lg border px-4 py-3 space-y-2 transition ${
                task.status === 'running'
                  ? 'border-purple-300 dark:border-purple-700 shadow-purple-100 shadow-sm'
                  : task.status === 'error'
                  ? 'border-red-200 dark:border-red-800'
                  : task.status === 'done'
                  ? 'border-green-200 dark:border-green-800'
                  : 'border-gray-200 dark:border-gray-700'
              }`}
            >
              <div className="flex items-center gap-2">
                {task.status === 'pending' && <div className="w-4 h-4 rounded-full border-2 border-gray-300" />}
                {task.status === 'running' && <Loader2 size={16} className="animate-spin text-purple-500" />}
                {task.status === 'done' && <Check size={16} className="text-green-500" />}
                {task.status === 'error' && <X size={16} className="text-red-500" />}
                <span className="text-sm font-medium">
                  [{task.id}] {task.prompt.length > 60 ? task.prompt.slice(0, 60) + '...' : task.prompt}
                </span>
                <span className="ml-auto text-xs text-gray-400">
                  {task.status === 'pending' && '等待中'}
                  {task.status === 'running' && '执行中…'}
                  {task.status === 'done' && '完成'}
                  {task.status === 'error' && '出错'}
                </span>
              </div>
              {task.result && (
                <pre className="text-xs text-gray-600 dark:text-gray-300 whitespace-pre-wrap bg-gray-50 dark:bg-gray-800 rounded p-2 max-h-40 overflow-y-auto">
                  {task.result}
                </pre>
              )}
              {task.error && (
                <pre className="text-xs text-red-500 whitespace-pre-wrap bg-red-50 dark:bg-red-900/10 rounded p-2">
                  {task.error}
                </pre>
              )}
            </div>
          ))}

          {/* 汇总 */}
          {summary && (
            <div className="bg-purple-50 dark:bg-purple-900/10 rounded-lg border border-purple-200 dark:border-purple-800 px-4 py-3">
              <h3 className="text-sm font-semibold mb-2 flex items-center gap-1">
                <Check size={16} className="text-purple-500" /> 汇总
              </h3>
              <pre className="text-sm whitespace-pre-wrap">{summary}</pre>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
