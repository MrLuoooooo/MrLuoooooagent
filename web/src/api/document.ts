import { apiFetch } from './client'
import type { DocumentItem } from '../types/document'

/** 上传文档 */
export function uploadDocument(file: File, title?: string): Promise<DocumentItem> {
  const form = new FormData()
  form.append('file', file)
  if (title) form.append('title', title)
  return apiFetch('/documents', {
    method: 'POST',
    body: form,
  })
}

/** 获取文档列表 */
export function listDocuments(): Promise<{
  total: number
  documents: DocumentItem[]
}> {
  return apiFetch('/documents')
}
