import type { StreamEvent } from '../types/chat'
import { rawFetch } from './client'
import { parseSSEStream } from '../lib/sse-parser'

export function chatStream(
  conversationId: string,
  message: string,
  agent: boolean,
  onEvent: (evt: StreamEvent) => void,
  onError: (err: Error) => void,
  onFinally: () => void,
  stockMode?: boolean,
): AbortController {
  const controller = new AbortController()

  ;(async () => {
    try {
      const res = await rawFetch('/chat', {
        method: 'POST',
        signal: controller.signal,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          conversation_id: conversationId,
          question: message,
          stream: true,
          agent,
          stock_mode: stockMode || false,
        }),
      })

      if (!res.ok) {
        const errBody = await res.text()
        onError(new Error(`HTTP ${res.status}: ${errBody}`))
        return
      }

      const reader = res.body?.getReader()
      if (!reader) {
        onError(new Error('响应体为空'))
        return
      }

      await parseSSEStream(reader, (line) => {
        try {
          const evt = JSON.parse(line) as StreamEvent
          onEvent(evt)
        } catch {}
      })
    } catch (err) {
      if (err instanceof Error && err.name !== 'AbortError') {
        onError(err)
      }
    } finally {
      onFinally()
    }
  })()

  return controller
}
