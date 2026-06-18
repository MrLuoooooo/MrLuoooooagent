// 股票相关类型

export interface KLineItem {
  code: string
  time: string
  open: number
  high: number
  low: number
  close: number
  volume: number
}

export interface StockRealtime {
  code: string
  name: string
  price: number
  open: number
  high: number
  low: number
  pre_close: number
  change: number
  change_rate: number
  volume: number
  amount: number
  source: string
}

export interface StockBasic {
  code: string
  name: string
  industry: string
  market_cap: number
  pe: number
  pb: number
}
