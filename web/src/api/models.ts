import { apiFetch } from './client'
import type { ModelItem, CustomModelForm } from '../types/models'

export function fetchModels(): Promise<ModelItem[]> {
  return apiFetch<ModelItem[]>('/models')
}

export function switchModel(modelName: string): Promise<{ model: string; message: string }> {
  return apiFetch('/models/switch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ model: modelName }),
  })
}

export function addCustomModel(form: CustomModelForm): Promise<{ name: string; message: string }> {
  return apiFetch('/models', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(form),
  })
}

export function removeCustomModel(name: string): Promise<void> {
  return apiFetch(`/models/${encodeURIComponent(name)}`, { method: 'DELETE' })
}

