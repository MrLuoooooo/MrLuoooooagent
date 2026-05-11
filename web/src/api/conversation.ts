import { apiFetch } from './client'
import type { ConversationItem, MessageItem } from '../types/conversation'

/** 创建会话 */
export function createConversation(title: string): Promise<ConversationItem> {
  return apiFetch('/conversations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  })
}

/** 获取会话列表 */
export function listConversations(): Promise<{
  total: number
  conversations: ConversationItem[]
}> {
  return apiFetch('/conversations')
}

/** 获取会话消息 */
export function getConversationMessages(id: string): Promise<{
  conversation_id: string
  total: number
  messages: MessageItem[]
}> {
  return apiFetch(`/conversations/${id}/messages`)
}

/** 删除会话 */
export function deleteConversation(id: string): Promise<{ conversation_id: string }> {
  return apiFetch(`/conversations/${id}`, {
    method: 'DELETE',
  })
}
