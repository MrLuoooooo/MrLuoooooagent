import { useState, useRef, useCallback, useEffect } from 'react'
import type { Message, ChatStatus, ToolCall, StreamEvent } from '../types/chat'
import { chatStream } from '../api/chat'

interface UseChatStreamReturn {
  messages: Message[]
  status: ChatStatus
  error: string | null
  toolCalls: ToolCall[]
  sendMessage: (conversationId: string, text: string, agent?: boolean) => void
  cancel: () => void
  clear: () => void
  setInitialMessages: (msgs: Message[]) => void
  setOnDone: (cb: (() => void) | null) => void
}

let msgCounter = 0
let streamIdCounter = 0
function nextId(): string {
  return `msg_${++msgCounter}_${Date.now()}`
}
function nextStreamId(): number {
  return ++streamIdCounter
}

export function useChatStream(): UseChatStreamReturn {
  const [messages, setMessages] = useState<Message[]>([])
  const [status, setStatus] = useState<ChatStatus>('idle')
  const [error, setError] = useState<string | null>(null)
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([])
  const abortRef = useRef<AbortController | null>(null)
  const statusRef = useRef<ChatStatus>('idle')
  const streamIdRef = useRef(0)

  useEffect(() => { statusRef.current = status }, [status])

  const currentAssistantRef = useRef<Message | null>(null)
  const onNewConversationRef = useRef<((id: string) => void) | null>(null)
  const onDoneRef = useRef<(() => void) | null>(null)

  const setOnDone = useCallback((cb: (() => void) | null) => {
    onDoneRef.current = cb
  }, [])

  const setInitialMessages = useCallback((msgs: Message[]) => {
    streamIdRef.current = 0
    setMessages(msgs)
    setStatus('idle')
    setError(null)
    setToolCalls([])
  }, [])

  const sendMessage = useCallback((conversationId: string, text: string, agent?: boolean) => {
    if (!text.trim() || statusRef.current === 'streaming') return

    const sid = nextStreamId()
    streamIdRef.current = sid

    setError(null)
    setStatus('streaming')
    setToolCalls([])
    currentAssistantRef.current = null

    const userMsg: Message = {
      id: nextId(),
      role: 'user',
      content: text,
      createdAt: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, userMsg])

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
      agent ?? false,
      (evt: StreamEvent) => {
        if (streamIdRef.current !== sid) return

        switch (evt.type) {
          case 'token':
            setMessages((prev) => {
              if (prev.length === 0) return prev
              const lastIdx = prev.length - 1
              const last = prev[lastIdx]
              if (last.role !== 'assistant') return prev
              return prev.map((msg, i) =>
                i === lastIdx
                  ? { ...msg, content: msg.content + (evt.content ?? '') }
                  : msg,
              )
            })
            break

          case 'conversation_id':
            if (evt.content && onNewConversationRef.current) {
              onNewConversationRef.current(evt.content)
            }
            break

          case 'tool_call':
            if (streamIdRef.current !== sid) return
            setStatus('tool_calling')
            setToolCalls((prev) => [
              ...prev,
              { name: evt.tool_name ?? evt.tool ?? '', args: evt.tool_args ?? '', status: 'running' },
            ])
            break

          case 'tool_result':
            if (streamIdRef.current !== sid) return
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
            if (streamIdRef.current !== sid) return
            setStatus('idle')
            onDoneRef.current?.()
            break

          case 'waiting':
            // 排队进度：在最后一条 assistant 消息中显示排队信息
            setMessages((prev) => {
              if (prev.length === 0) return prev
              const lastIdx = prev.length - 1
              return prev.map((msg, i) =>
                i === lastIdx && msg.role === 'assistant'
                  ? { ...msg, content: '⏳ ' + (evt.content ?? '排队中...') }
                  : msg,
              )
            })
            break

          case 'sources':
            // 引用来源：附加到当前 assistant 消息，渲染"参考来源"区块
            if (streamIdRef.current !== sid) return
            setMessages((prev) => {
              if (prev.length === 0) return prev
              const lastIdx = prev.length - 1
              return prev.map((msg, i) =>
                i === lastIdx && msg.role === 'assistant'
                  ? { ...msg, sources: evt.sources ?? [] }
                  : msg,
              )
            })
            break

          case 'error':
            if (streamIdRef.current !== sid) return
            setError(evt.content ?? '未知错误')
            setStatus('error')
            break
        }
      },
      (err: Error) => {
        if (streamIdRef.current !== sid) return
        setError(err.message)
        setStatus('error')
      },
      () => {},
    )
  }, [])

  const cancel = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
    setStatus('idle')
  }, [])

  const clear = useCallback(() => {
    streamIdRef.current = 0
    setMessages([])
    setError(null)
    setStatus('idle')
    setToolCalls([])
  }, [])

  return {
    messages,
    status,
    error,
    toolCalls,
    sendMessage,
    cancel,
    clear,
    setInitialMessages,
    setOnDone,
  }
}

export type { UseChatStreamReturn }
