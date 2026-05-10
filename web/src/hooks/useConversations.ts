import { useState, useEffect, useCallback } from 'react'
import type { ConversationItem } from '../types/conversation'
import {
  createConversation as apiCreate,
  listConversations as apiList,
} from '../api/conversation'

interface UseConversationsReturn {
  conversations: ConversationItem[]
  loading: boolean
  error: string | null
  refresh: () => void
  create: (title?: string) => Promise<string | null>
  currentId: string | null
  setCurrentId: (id: string | null) => void
}

export function useConversations(): UseConversationsReturn {
  const [conversations, setConversations] = useState<ConversationItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentId, setCurrentId] = useState<string | null>(null)

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
      setConversations((prev) => [item as unknown as ConversationItem, ...prev])
      const id = (item as unknown as { conversation_id: string }).conversation_id
      setCurrentId(id)
      return id
    } catch (err) {
      setError((err as Error).message)
      return null
    }
  }, [])

  return {
    conversations,
    loading,
    error,
    refresh,
    create,
    currentId,
    setCurrentId,
  }
}
