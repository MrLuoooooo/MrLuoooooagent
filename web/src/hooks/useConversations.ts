import { useState, useEffect, useCallback } from 'react'
import type { ConversationItem, MessageItem } from '../types/conversation'
import {
  createConversation as apiCreate,
  listConversations as apiList,
  getConversationMessages as apiGetMessages,
  deleteConversation as apiDelete,
} from '../api/conversation'

interface UseConversationsReturn {
  conversations: ConversationItem[]
  loading: boolean
  error: string | null
  refresh: () => void
  create: (title?: string) => Promise<string | null>
  delete: (id: string) => Promise<boolean>
  currentId: string | null
  setCurrentId: (id: string | null) => void
  messages: MessageItem[]
  loadMessages: (id: string) => Promise<MessageItem[]>
  messagesLoading: boolean
}

export function useConversations(): UseConversationsReturn {
  const [conversations, setConversations] = useState<ConversationItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentId, setCurrentId] = useState<string | null>(null)
  const [messages, setMessages] = useState<MessageItem[]>([])
  const [messagesLoading, setMessagesLoading] = useState(false)

  const refresh = useCallback(() => {
    setLoading(true)
    apiList()
      .then((data) => {
        setConversations(data.conversations)
        setError(null)
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const create = useCallback(async (title?: string): Promise<string | null> => {
    try {
      const item = await apiCreate(title ?? '新会话')
      const id = (item as unknown as { conversation_id: string }).conversation_id
      setConversations((prev) => [item as unknown as ConversationItem, ...prev])
      setCurrentId(id)
      setMessages([])
      return id
    } catch (err) {
      setError((err as Error).message)
      return null
    }
  }, [])

  const deleteConv = useCallback(async (id: string): Promise<boolean> => {
    try {
      await apiDelete(id)
      setConversations((prev) => prev.filter((c) => c.conversation_id !== id))
      if (currentId === id) {
        setCurrentId(null)
        setMessages([])
      }
      return true
    } catch (err) {
      setError((err as Error).message)
      return false
    }
  }, [currentId])

  const loadMessages = useCallback(async (id: string): Promise<MessageItem[]> => {
    setMessagesLoading(true)
    try {
      const data = await apiGetMessages(id)
      setMessages(data.messages)
      return data.messages
    } catch (err) {
      setError((err as Error).message)
      setMessages([])
      return []
    } finally {
      setMessagesLoading(false)
    }
  }, [])

  return {
    conversations,
    loading,
    error,
    refresh,
    create,
    delete: deleteConv,
    currentId,
    setCurrentId,
    messages,
    loadMessages,
    messagesLoading,
  }
}
