import { apiFetch } from './client'
import type { ConversationItem, MessageItem } from '../types/conversation'

export function createConversation(title: string): Promise<ConversationItem> {
  return apiFetch('/conversations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  })
}

export function listConversations(): Promise<{
  total: number
  conversations: ConversationItem[]
}> {
  return apiFetch('/conversations')
}

export function getConversationMessages(id: string): Promise<{
  conversation_id: string
  total: number
  messages: MessageItem[]
}> {
  return apiFetch(`/conversations/${id}/messages`)
}

export function deleteConversation(id: string): Promise<{ conversation_id: string }> {
  return apiFetch(`/conversations/${id}`, {
    method: 'DELETE',
  })
}

export function deleteAllConversations(): Promise<void> {
  return apiFetch('/conversations', { method: 'DELETE' })
}
