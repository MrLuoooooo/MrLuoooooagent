import { useState, useRef, useCallback } from 'react'
import type { Message, ChatStatus, ToolCall } from '../types/chat'
import { chatStream } from '../api/chat'

interface UseChatStreamReturn {
  messages: Message[]
  status: ChatStatus
  error: string | null
  toolCalls: ToolCall[]
  sendMessage: (conversationId: string, text: string) => void
  cancel: () => void
  clear: () => void
}

let msgCounter = 0
function nextId(): string {
  return `msg_${++msgCounter}_${Date.now()}`
}

export function useChatStream(): UseChatStreamReturn {
  const [messages, setMessages] = useState<Message[]>([])
  const [status, setStatus] = useState<ChatStatus>('idle')
  const [error, setError] = useState<string | null>(null)
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([])
  const abortRef = useRef<AbortController | null>(null)

  const currentAssistantRef = useRef<Message | null>(null)

  const sendMessage = useCallback((conversationId: string, text: string) => {
    if (!text.trim() || status === 'streaming') return

    setError(null)
    setStatus('streaming')
    setToolCalls([])
    currentAssistantRef.current = null

    // 添加用户消息
    const userMsg: Message = {
      id: nextId(),
      role: 'user',
      content: text,
      createdAt: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, userMsg])

    // 添加空的 assistant 消息占位
    const assistantMsg: Message = {
      id: nextId(),
      role: 'assistant',
      content: '',
      createdAt: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, assistantMsg])
    currentAssistantRef.current = assistantMsg

    abortRef.current = chatStream(
      conversationId,
      text,
      (evt) => {
        switch (evt.type) {
          case 'token':
            setMessages((prev) => {
              if (prev.length === 0) return prev
              const lastIdx = prev.length - 1
              const last = prev[lastIdx]
              if (last.role !== 'assistant') return prev
              // 不原地修改，创建新对象避免 StrictMode 双调用重复追加
              return prev.map((msg, i) =>
                i === lastIdx
                  ? { ...msg, content: msg.content + (evt.content ?? '') }
                  : msg,
              )
            })
            break

          case 'tool_call':
            setStatus('tool_calling')
            setToolCalls((prev) => [
              ...prev,
              { tool: evt.tool ?? '', status: 'running' },
            ])
            break

          case 'tool_result':
            setToolCalls((prev) => {
              const clone = [...prev]
              const last = clone[clone.length - 1]
              if (last) {
                last.status = 'done'
                last.output = evt.content
              }
              return [...clone]
            })
            setStatus('streaming')
            break

          case 'done':
            setStatus('idle')
            break

          case 'error':
            setError(evt.content ?? '未知错误')
            setStatus('error')
            break
        }
      },
      (err) => {
        setError(err.message)
        setStatus('error')
      },
      () => {
        // onFinally: 不在状态机忙时重设状态
      },
    )
  }, [status])

  const cancel = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
    setStatus('idle')
  }, [])

  const clear = useCallback(() => {
    setMessages([])
    setError(null)
    setStatus('idle')
    setToolCalls([])
  }, [])

  return { messages, status, error, toolCalls, sendMessage, cancel, clear }
}
