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
  message?: string
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

const DEV_BASE = 'http://127.0.0.1:8081/api/v1'

export function importMcpZip(name: string, file: File): Promise<McpImportResult> {
  const form = new FormData()
  form.append('name', name)
  form.append('file', file)

  const BASE = import.meta.env.DEV ? DEV_BASE : '/api/v1'
  const token = localStorage.getItem('goagent_token') || 'dev-token'

  return fetch(BASE + '/mcp/import', {
    method: 'POST',
    headers: { 'Authorization': 'Bearer ' + token },
    body: form,
  }).then(async r => {
    const body = await r.json()
    if (!r.ok) throw new Error(body.message || body.error || `HTTP ${r.status}`)
    if (body.code !== 0) throw new Error(body.message || body.error || 'import failed')
    return body as McpImportResult
  })
}
