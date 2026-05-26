import { apiFetch } from './client'

export interface ApprovalItem {
  id: string
  created_at: string
  source: string
  task_name: string
  action_type: string
  risk_level: string
  reason: string
  prompt: string
  full_output: string
  status: 'pending' | 'accepted' | 'rejected' | 'expired'
  approved_at?: string
}

export function fetchPendingApprovals(): Promise<ApprovalItem[]> {
  return apiFetch<ApprovalItem[]>('/approvals/pending')
}

export function fetchAllApprovals(): Promise<ApprovalItem[]> {
  return apiFetch<ApprovalItem[]>('/approvals')
}

export function decideApproval(id: string, action: 'accept' | 'reject' | 'skip' | boolean): Promise<void> {
  const body = typeof action === 'boolean'
    ? { accept: action }
    : { action }
  return apiFetch<void>(`/approvals/${id}/decide`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}
