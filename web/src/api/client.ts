import type { APIEnvelope } from '../types/envelope'

const TOKEN_KEY = 'goagent_token'
const TOKEN = import.meta.env.VITE_API_TOKEN || localStorage.getItem(TOKEN_KEY) || 'dev-token'
const BASE = '/api/v1'
export { BASE }

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || TOKEN
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...init?.headers,
      Authorization: `Bearer ${getToken()}`,
    },
  })

  const body = (await res.json()) as APIEnvelope<T>

  if (body.code !== 0) {
    throw new Error(body.message || `请求失败 (${res.status})`)
  }

  return body.data as T
}

export function rawFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...init?.headers,
      Authorization: `Bearer ${getToken()}`,
    },
  })
}
