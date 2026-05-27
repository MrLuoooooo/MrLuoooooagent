export interface ChatRequest {
  conversation_id: string
  question: string
  stream: boolean
  agent?: boolean
}

export interface StreamEvent {
  type: 'token' | 'tool_call' | 'tool_result' | 'done' | 'error' | 'conversation_id'
  content?: string
  tool?: string
  tool_name?: string
  tool_args?: string
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  toolCalls?: ToolCall[]
  createdAt: string
}

export interface ToolCall {
  name: string
  args: string
  status: 'pending' | 'running' | 'done' | 'error'
  input?: string
  output?: string
}

export type ChatStatus = 'idle' | 'streaming' | 'tool_calling' | 'error'
