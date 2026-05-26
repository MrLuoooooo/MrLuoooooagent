import { apiFetch } from './client'
import type { SkillItem } from '../types/models'

export function fetchSkills(): Promise<SkillItem[]> {
  return apiFetch<SkillItem[]>('/skills')
}

export function upsertSkill(skill: SkillItem): Promise<{ name: string; message: string }> {
  return apiFetch('/skills', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(skill),
  })
}

export function removeSkill(name: string): Promise<void> {
  return apiFetch(`/skills/${encodeURIComponent(name)}`, { method: 'DELETE' })
}
