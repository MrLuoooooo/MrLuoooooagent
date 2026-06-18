import { useState, useEffect, useCallback } from 'react'
import { Search, Plus, X, TrendingUp, TrendingDown } from 'lucide-react'
import { searchStocks, fetchRealtime, getWatchlist, addToWatchlist, removeFromWatchlist } from '../api/stock'
import type { StockBasic, StockRealtime } from '../types/stock'

interface Props {
  selectedCode: string
  onSelect: (code: string) => void
}

interface WatchItem {
  code: string
  info: StockRealtime | null
}

export default function StockSearch({ selectedCode, onSelect }: Props) {
  const [keyword, setKeyword] = useState('')
  const [results, setResults] = useState<StockBasic[]>([])
  const [watchlist, setWatchlist] = useState<WatchItem[]>([])
  const [searching, setSearching] = useState(false)

  const loadWatchlist = useCallback(async () => {
    try {
      const codes = await getWatchlist()
      const items: WatchItem[] = []
      for (const code of codes) {
        try {
          const info = await fetchRealtime(code)
          items.push({ code, info })
        } catch {
          items.push({ code, info: null })
        }
      }
      setWatchlist(items)
    } catch {}
  }, [])

  useEffect(() => { loadWatchlist() }, [loadWatchlist])

  const handleSearch = async () => {
    if (!keyword.trim()) return
    setSearching(true)
    try {
      const r = await searchStocks(keyword)
      setResults(r.slice(0, 8))
    } catch {
      setResults([])
    } finally {
      setSearching(false)
    }
  }

  const handleAdd = async (code: string) => {
    await addToWatchlist(code)
    setResults([])
    setKeyword('')
    loadWatchlist()
  }

  const handleRemove = async (code: string) => {
    await removeFromWatchlist(code)
    loadWatchlist()
  }

  return (
    <div className="flex flex-col gap-3 h-full">
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-500" />
          <input
            type="text"
            placeholder="搜索股票..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            className="w-full pl-8 pr-3 py-1.5 rounded-lg border border-gray-700 bg-gray-800 text-gray-200 text-sm focus:outline-none focus:border-blue-500"
          />
        </div>
        <button
          onClick={handleSearch}
          disabled={searching}
          className="px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium disabled:opacity-50"
        >
          {searching ? '...' : '搜索'}
        </button>
      </div>

      {results.length > 0 && (
        <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
          {results.map((r) => (
            <button
              key={r.code}
              onClick={() => handleAdd(r.code)}
              className="w-full flex items-center justify-between px-3 py-2 hover:bg-gray-750 border-b border-gray-700 last:border-0 text-left"
            >
              <div>
                <span className="text-gray-200 text-sm font-medium">{r.name}</span>
                <span className="text-gray-500 text-xs ml-2">{r.code}</span>
              </div>
              <span className="text-gray-400 text-xs">{r.industry}</span>
              <Plus size={14} className="text-blue-400" />
            </button>
          ))}
        </div>
      )}

      <div className="flex-1 overflow-y-auto space-y-1">
        <p className="text-gray-500 text-xs font-medium px-1 py-1">自选股</p>
        {watchlist.length === 0 && (
          <p className="text-gray-600 text-xs px-1">搜索股票添加自选</p>
        )}
        {watchlist.map((w) => (
          <button
            key={w.code}
            onClick={() => onSelect(w.code)}
            className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-left transition-colors ${
              w.code === selectedCode
                ? 'bg-blue-600/20 border border-blue-500/50'
                : 'hover:bg-gray-800 border border-transparent'
            }`}
          >
            <div className="flex-1 min-w-0">
              <div className="text-gray-200 text-sm truncate">
                {w.info?.name || w.code}
              </div>
              <div className="text-gray-500 text-xs">{w.code}</div>
            </div>
            {w.info && (
              <div className="text-right flex-shrink-0">
                <div className="text-sm font-mono text-gray-200">
                  ¥{w.info.price.toFixed(2)}
                </div>
                <div className={`text-xs flex items-center gap-0.5 ${w.info.change_rate >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                  {w.info.change_rate >= 0 ? <TrendingUp size={10} /> : <TrendingDown size={10} />}
                  {w.info.change_rate >= 0 ? '+' : ''}{w.info.change_rate.toFixed(2)}%
                </div>
              </div>
            )}
            <X
              size={14}
              className="text-gray-600 hover:text-red-400 ml-2 flex-shrink-0"
              onClick={(e) => { e.stopPropagation(); handleRemove(w.code) }}
            />
          </button>
        ))}
      </div>
    </div>
  )
}
