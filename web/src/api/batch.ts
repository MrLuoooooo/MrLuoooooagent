import { rawFetch } from './client'

export interface BatchTask {
  id: string
  prompt: string
}

export interface BatchProgress {
  type: 'task_start' | 'task_token' | 'task_done' | 'task_error' | 'summary' | 'done'
  task_id?: string
  result?: string
  error?: string
}

export function batchStream(
  tasks: BatchTask[],
  agent: boolean,
  onEvent: (evt: BatchProgress) => void,
  onError: (err: Error) => void,
  onFinally: () => void,
): AbortController {
  const controller = new AbortController()

  ;(async () => {
    try {
      const res = await rawFetch('/batch', {
        method: 'POST',
        signal: controller.signal,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tasks, agent }),
      })

      if (!res.ok) {
        const errBody = await res.text()
        onError(new Error(`HTTP ${res.status}: ${errBody}`))
        return
      }

      const reader = res.body?.getReader()
      if (!reader) { onError(new Error('响应体为空')); return }

      const decoder = new TextDecoder()
      let buf = ''
      let reading = true
      while (reading) {
        const { done, value } = await reader.read()
        if (done) break
        buf += decoder.decode(value, { stream: true })
        const lines = buf.split('\n\n')
        buf = lines.pop() || ''
        for (const line of lines) {
          const trimmed = line.trim()
          if (trimmed.startsWith('data: ')) {
            try {
              const evt = JSON.parse(trimmed.slice(6)) as BatchProgress
              onEvent(evt)
              if (evt.type === 'done') reading = false
            } catch { /* skip */ }
          }
        }
      }
    } catch (err) {
      if (err instanceof Error && err.name !== 'AbortError') onError(err)
    } finally {
      onFinally()
    }
  })()

  return controller
}
