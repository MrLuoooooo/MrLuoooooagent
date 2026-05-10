/**
 * SSE 流式解析工具。
 * 手动拼接 buffer 处理中文字符截断，按 \n\n 分割事件。
 */

export async function parseSSEStream(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  onLine: (line: string) => void,
): Promise<void> {
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })

    // 按 \n\n 分割完整事件块
    const parts = buffer.split('\n\n')
    // 最后一个可能是不完整的块，保留到下次
    buffer = parts.pop() ?? ''

    for (const part of parts) {
      const trimmed = part.trim()
      if (!trimmed) continue

      // 提取 data: 前缀后的内容
      for (const line of trimmed.split('\n')) {
        if (line.startsWith('data: ')) {
          const payload = line.slice(6)
          if (payload === '[DONE]') continue
          onLine(payload)
        }
      }
    }
  }

  // 处理最后残留的 buffer
  if (buffer.trim()) {
    for (const line of buffer.trim().split('\n')) {
      if (line.startsWith('data: ')) {
        const payload = line.slice(6)
        if (payload !== '[DONE]') onLine(payload)
      }
    }
  }
}
