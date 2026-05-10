/** 会话项 */
export interface ConversationItem {
  conversation_id: string
  title: string
  message_count: number
  created_at: string
  updated_at: string
}

/** 消息条目 */
export interface MessageItem {
  role: string
  content: string
}
