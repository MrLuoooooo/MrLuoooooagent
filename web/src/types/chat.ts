/** 聊天请求体（匹配后端 model/chat.go ChatRequest） */
export interface ChatRequest {
  conversation_id: string
  question: string
  stream: boolean
  agent?: boolean
}

/** SSE 流式事件 */
export interface StreamEvent {
  type: 'token' | 'tool_call' | 'tool_result' | 'done' | 'error' | 'conversation_id'
  content?: string
  tool?: string
  tool_name?: string
  tool_args?: string
}

/** 对话消息 */
export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  toolCalls?: ToolCall[]
  createdAt: string
}

/** 工具调用记录 */
export interface ToolCall {
  name: string
  args: string
  status: 'pending' | 'running' | 'done' | 'error'
  input?: string
  output?: string
}

/** 聊天状态 */
export type ChatStatus = 'idle' | 'streaming' | 'tool_calling' | 'error'
