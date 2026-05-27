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

    const parts = buffer.split('\n\n')
    buffer = parts.pop() ?? ''

    for (const part of parts) {
      const trimmed = part.trim()
      if (!trimmed) continue

      for (const line of trimmed.split('\n')) {
        if (line.startsWith('data: ')) {
          const payload = line.slice(6)
          if (payload === '[DONE]') continue
          onLine(payload)
        }
      }
    }
  }

  if (buffer.trim()) {
    for (const line of buffer.trim().split('\n')) {
      if (line.startsWith('data: ')) {
        const payload = line.slice(6)
        if (payload !== '[DONE]') onLine(payload)
      }
    }
  }
}
