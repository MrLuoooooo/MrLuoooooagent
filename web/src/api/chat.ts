import type { StreamEvent } from '../types/chat'
import { rawFetch } from './client'
import { parseSSEStream } from '../lib/sse-parser'

/**
 * 发送聊天请求并 SSE 流式读取响应。
 * onEvent 回调每次收到 StreamEvent 时触发。
 * 返回 AbortController 用于取消。
 */
export function chatStream(
  conversationId: string,
  message: string,
  onEvent: (evt: StreamEvent) => void,
  onError: (err: Error) => void,
  onFinally: () => void,
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
        } catch {
          // 跳过解析失败的事件
        }
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
