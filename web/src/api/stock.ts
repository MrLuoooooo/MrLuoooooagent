import { rawFetch } from './client'
import type { KLineItem, StockRealtime, StockBasic } from '../types/stock'

export async function fetchKLine(code: string, period = 'day', limit = 120): Promise<KLineItem[]> {
  const resp = await rawFetch(`/stock/kline?code=${code}&period=${period}&limit=${limit}`)
  const json = await resp.json()
  if (json.code !== 0) throw new Error(json.message)
  return json.data as KLineItem[]
}

export async function fetchRealtime(code: string): Promise<StockRealtime> {
  const resp = await rawFetch(`/stock/realtime?code=${code}`)
  const json = await resp.json()
  if (json.code !== 0) throw new Error(json.message)
  return json.data as StockRealtime
}

export async function searchStocks(keyword: string): Promise<StockBasic[]> {
  const resp = await rawFetch(`/stock/search?keyword=${encodeURIComponent(keyword)}`)
  const json = await resp.json()
  if (json.code !== 0) throw new Error(json.message)
  return (json.data || []) as StockBasic[]
}

export async function getWatchlist(): Promise<string[]> {
  const resp = await rawFetch(`/stock/watchlist`)
  const json = await resp.json()
  return (json.data || []) as string[]
}

export async function addToWatchlist(code: string): Promise<void> {
  await rawFetch(`/stock/watchlist`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  })
}

export async function removeFromWatchlist(code: string): Promise<void> {
  await rawFetch(`/stock/watchlist/${code}`, { method: 'DELETE' })
}
