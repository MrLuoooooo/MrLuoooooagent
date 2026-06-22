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

export function fetchMcpServers(): Promise<McpConfig> {
  return apiFetch<McpConfig>('/mcp/servers')
}

export function upsertMcpServer(server: McpServer): Promise<void> {
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
