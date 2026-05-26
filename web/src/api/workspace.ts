import { apiFetch } from './client'

interface WorkspaceResponse {
  path: string
}

export function fetchWorkspace(): Promise<WorkspaceResponse> {
  return apiFetch<WorkspaceResponse>('/workspace')
}

export function setWorkspace(path: string): Promise<WorkspaceResponse> {
  return apiFetch<WorkspaceResponse>('/workspace', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
}
