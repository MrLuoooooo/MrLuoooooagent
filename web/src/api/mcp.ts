import { apiFetch } from './client'

export interface McpServer {
  name: string
  transport: 'sse' | 'stdio'
  url?: string
  command?: string
  args?: string[]
  env?: Record<string, string>
}

export interface McpConfig {
  enabled: boolean
  servers: McpServer[]
}

export interface McpImportResult {
  code: number
  server?: McpServer
  connected?: boolean
  tool_count?: number
  error?: string
}

export function fetchMcpServers(): Promise<McpConfig> {
  return apiFetch<McpConfig>('/mcp/servers')
}

export function upsertMcpServer(server: McpServer): Promise<{ connected?: boolean; tool_count?: number; error?: string }> {
  return apiFetch('/mcp/servers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(server),
  })
}

export function removeMcpServer(name: string): Promise<void> {
  return apiFetch(`/mcp/servers/${encodeURIComponent(name)}`, { method: 'DELETE' })
}

export function toggleMcpEnabled(enabled: boolean): Promise<void> {
  return apiFetch('/mcp/enabled', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
}

export function importMcpZip(name: string, file: File): Promise<McpImportResult> {
  const form = new FormData()
  form.append('name', name)
  form.append('file', file)

  const BASE = import.meta.env.DEV ? 'http://127.0.0.1:8080/api/v1' : '/api/v1'
  const token = import.meta.env.VITE_API_TOKEN || localStorage.getItem('goagent_token') || 'dev-token'

  return fetch(BASE + '/mcp/import', {
    method: 'POST',
    headers: { 'Authorization': 'Bearer ' + token },
    body: form,
  }).then(r => r.json())
}
