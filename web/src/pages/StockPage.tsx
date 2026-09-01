import { useState, useEffect, useCallback, useRef } from 'react'
import { Send, Bot, TrendingUp, TrendingDown, BarChart3 } from 'lucide-react'
import StockChart from '../components/StockChart'
import StockSearch from '../components/StockSearch'
import { fetchKLine, fetchRealtime } from '../api/stock'
import type { KLineItem, StockRealtime } from '../types/stock'
import { chatStream } from '../api/chat'
import type { StreamEvent } from '../types/chat'

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  toolCalls?: { name: string; args: string; result?: string; status: 'running' | 'done' }[]
}

export default function StockPage() {
  const [selectedCode, setSelectedCode] = useState('sh600519')
  const [klineData, setKlineData] = useState<KLineItem[]>([])
  const [realtime, setRealtime] = useState<StockRealtime | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [period, setPeriod] = useState('day')
  const [klineLoading, setKlineLoading] = useState(false)
  const [klineError, setKlineError] = useState<string | null>(null)
  const messagesEnd = useRef<HTMLDivElement>(null)

  const loadData = useCallback(async (code: string) => {
    // K 线与实时行情分开处理：行情失败可以静默（页面还有 K 线可看），
    // K 线失败必须显式提示——静默吞错曾导致"按钮点了没反应"却查不到原因。
    setKlineLoading(true)
    setKlineError(null)
    try {
      const klines = await fetchKLine(code, period, 120)
      setKlineData(klines)
    } catch (e) {
      setKlineData([])
      setKlineError(e instanceof Error ? e.message : 'K线数据加载失败')
    } finally {
      setKlineLoading(false)
    }
    fetchRealtime(code).then(setRealtime).catch(() => {})
  }, [period])

  useEffect(() => {
    if (selectedCode) loadData(selectedCode)
  }, [selectedCode, loadData])

  useEffect(() => {
    messagesEnd.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSend = async () => {
    if (!input.trim() || streaming) return
    const q = input.trim()
    setInput('')
    setMessages((prev) => [...prev, { role: 'user', content: q }])

    const assistMsg: ChatMessage = { role: 'assistant', content: '' }
    setMessages((prev) => [...prev, assistMsg])
    setStreaming(true)

    try {
      await chatStream(
        `stock_${selectedCode}`,
        q,
        true,
        (evt: StreamEvent) => {
          setMessages((prev) => {
            const updated = [...prev]
            const last = updated[updated.length - 1]
            if (!last || last.role !== 'assistant') return prev

            if (evt.type === 'token') {
              last.content += evt.content || ''
            } else if (evt.type === 'tool_call') {
              if (!last.toolCalls) last.toolCalls = []
              last.toolCalls.push({ name: evt.tool_name || 'unknown', args: evt.tool_args || '', status: 'running' })
            } else if (evt.type === 'tool_result') {
              if (last.toolCalls && last.toolCalls.length > 0) {
                const tc = last.toolCalls[last.toolCalls.length - 1]
                if (tc.status === 'running') {
                  tc.result = evt.content || ''
                  tc.status = 'done'
                }
              }
            }
            return updated
          })
        },
        () => {},
        () => setStreaming(false),
        true, // stock_mode
      )
    } catch {
      setStreaming(false)
    }
  }

  const periods = [
    { key: 'day', label: '日K' },
    { key: 'week', label: '周K' },
    { key: 'month', label: '月K' },
    { key: 'year', label: '年K' },
  ] as const

  return (
    <div className="flex h-full overflow-hidden">
      <div className="w-72 flex-shrink-0 border-r border-gray-800 bg-gray-950 p-3 flex flex-col gap-3 overflow-y-auto">
        <StockSearch selectedCode={selectedCode} onSelect={setSelectedCode} />
      </div>

      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <div className="border-b border-gray-800 bg-gray-950 px-4 py-2 flex items-center justify-between flex-shrink-0">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1.5">
              <BarChart3 size={16} className="text-blue-400" />
              <span className="text-gray-200 text-sm font-medium">股票分析</span>
            </div>
            {realtime && (
              <div className="flex items-center gap-3 text-sm">
                <span className="text-gray-300 font-mono">¥{realtime.price.toFixed(2)}</span>
                <span className={`flex items-center gap-0.5 font-mono ${realtime.change_rate >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                  {realtime.change_rate >= 0 ? <TrendingUp size={14} /> : <TrendingDown size={14} />}
                  {realtime.change_rate >= 0 ? '+' : ''}{realtime.change_rate.toFixed(2)}%
                </span>
              </div>
            )}
          </div>
          <div className="flex items-center gap-1">
            {periods.map((p) => (
              <button
                key={p.key}
                onClick={() => setPeriod(p.key)}
                className={`px-2.5 py-1 rounded text-xs font-medium transition-colors ${
                  p.key === period ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-gray-200'
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto">
          <div className="p-4">
            {klineError ? (
              <div className="flex flex-col items-center justify-center gap-2 h-64 rounded-lg border border-red-900/50 bg-red-950/20">
                <span className="text-sm text-red-400">K线加载失败：{klineError}</span>
                <button
                  onClick={() => loadData(selectedCode)}
                  className="px-3 py-1 rounded text-xs bg-blue-600 hover:bg-blue-700 text-white"
                >
                  重试
                </button>
              </div>
            ) : klineLoading ? (
              <div className="flex items-center justify-center h-64 text-gray-500 text-sm">
                加载K线数据中...
              </div>
            ) : (
              <StockChart data={klineData} code={selectedCode} />
            )}
          </div>

          <div className="px-4 pb-4 space-y-3">
            {messages.map((msg, i) => (
              <div
                key={i}
                className={`flex gap-2 ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                {msg.role === 'assistant' && (
                  <div className="w-7 h-7 rounded-lg bg-blue-600/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <Bot size={14} className="text-blue-400" />
                  </div>
                )}
                <div className={`max-w-[75%] rounded-xl px-3 py-2 text-sm ${
                  msg.role === 'user'
                    ? 'bg-blue-600 text-white rounded-br-md'
                    : 'bg-gray-800 text-gray-200 rounded-bl-md'
                }`}>
                  {msg.toolCalls?.map((tc, j) => (
                    <div key={j} className="text-xs text-purple-400 mb-1 border-b border-gray-700 pb-1">
                      🔧 {tc.name}({tc.args.slice(0, 60)}{tc.args.length > 60 ? '...' : ''})
                      {tc.result && <div className="text-gray-400 mt-0.5 truncate">{tc.result.slice(0, 80)}</div>}
                    </div>
                  ))}
                  {msg.content || (msg.role === 'assistant' && streaming ? (
                    <span className="inline-block w-1.5 h-4 bg-blue-400 animate-pulse rounded-sm align-middle" />
                  ) : null)}
                </div>
                {msg.role === 'user' && (
                  <div className="w-7 h-7 rounded-lg bg-blue-600 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <span className="text-white text-xs font-bold">U</span>
                  </div>
                )}
              </div>
            ))}
            <div ref={messagesEnd} />
          </div>
        </div>

        <div className="border-t border-gray-800 bg-gray-950 px-4 py-3 flex-shrink-0">
          <div className="flex items-center gap-2">
            <div className="flex items-center gap-1.5 px-2 py-1 rounded-lg bg-purple-600/20 border border-purple-500/50 flex-shrink-0">
              <Bot size={14} className="text-purple-400" />
              <span className="text-xs text-purple-300 font-medium">Agent</span>
            </div>
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSend()}
              placeholder="分析这只股票..."
              disabled={streaming}
              className="flex-1 px-3 py-1.5 rounded-lg border border-gray-700 bg-gray-800 text-gray-200 text-sm focus:outline-none focus:border-blue-500 disabled:opacity-50"
            />
            <button
              onClick={handleSend}
              disabled={streaming || !input.trim()}
              className="px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white disabled:opacity-50 flex-shrink-0"
            >
              <Send size={16} />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
