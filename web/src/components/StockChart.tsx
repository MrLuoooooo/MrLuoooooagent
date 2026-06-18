import { useEffect, useRef } from 'react'
import * as echarts from 'echarts'
import type { KLineItem } from '../types/stock'

interface Props {
  data: KLineItem[]
  code: string
}

export default function StockChart({ data, code }: Props) {
  const chartRef = useRef<HTMLDivElement>(null)
  const instanceRef = useRef<echarts.ECharts | null>(null)

  useEffect(() => {
    if (!chartRef.current || data.length === 0) return

    if (!instanceRef.current) {
      instanceRef.current = echarts.init(chartRef.current, 'dark')
    }
    const chart = instanceRef.current

    const dates = data.map((d) => d.time)
    const ohlc = data.map((d) => [d.open, d.close, d.low, d.high])
    const volumes = data.map((d) => d.volume)

    chart.setOption({
      backgroundColor: '#1a1a2e',
      title: {
        text: code.toUpperCase(),
        left: 12,
        top: 8,
        textStyle: { color: '#ccc', fontSize: 14 },
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
      },
      grid: [
        { left: '8%', right: '8%', top: 50, height: '55%' },
        { left: '8%', right: '8%', top: '75%', height: '15%' },
      ],
      xAxis: [
        {
          type: 'category',
          data: dates,
          axisLine: { lineStyle: { color: '#333' } },
          axisLabel: { show: false },
          gridIndex: 0,
        },
        {
          type: 'category',
          data: dates,
          axisLine: { lineStyle: { color: '#333' } },
          axisLabel: { 
            show: true, 
            color: '#888',
            fontSize: 10,
            formatter: (v: string) => {
              const parts = v.split('-')
              return parts.length === 3 ? `${parts[1]}/${parts[2]}` : v
            },
          },
          gridIndex: 1,
        },
      ],
      yAxis: [
        {
          type: 'value',
          scale: true,
          axisLine: { lineStyle: { color: '#333' } },
          axisLabel: { color: '#888', fontSize: 10, formatter: (v: number) => v.toFixed(2) },
          splitLine: { lineStyle: { color: '#222' } },
          gridIndex: 0,
        },
        {
          type: 'value',
          axisLabel: { show: false },
          splitLine: { show: false },
          gridIndex: 1,
        },
      ],
      series: [
        {
          name: 'K线',
          type: 'candlestick',
          data: ohlc,
          xAxisIndex: 0,
          yAxisIndex: 0,
          itemStyle: {
            color: '#ef5350',
            color0: '#26a69a',
            borderColor: '#ef5350',
            borderColor0: '#26a69a',
          },
        },
        {
          name: '成交量',
          type: 'bar',
          data: volumes,
          xAxisIndex: 1,
          yAxisIndex: 1,
          itemStyle: {
            color: (params: { dataIndex: number }) => {
              const item = data[params.dataIndex]
              return item && item.close >= item.open ? '#ef5350' : '#26a69a'
            },
          },
        },
      ],
    })

    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)
    return () => {
      window.removeEventListener('resize', handleResize)
    }
  }, [data, code])

  return (
    <div className="w-full rounded-xl overflow-hidden border border-gray-700 shadow-lg" style={{ height: 420 }}>
      <div ref={chartRef} className="w-full h-full" />
    </div>
  )
}
