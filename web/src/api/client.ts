import type { APIEnvelope } from '../types/envelope'

const TOKEN = 'dev-token'
const BASE = import.meta.env.DEV ? 'http://127.0.0.1:8080/api/v1' : '/api/v1'

/** 封装的 fetch，自动带 token 和错误处理 */
export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...init?.headers,
      Authorization: `Bearer ${TOKEN}`,
    },
  })

  // 非流式响应统一解包
  const body = (await res.json()) as APIEnvelope<T>

  if (body.code !== 0) {
    throw new Error(body.message || `请求失败 (${res.status})`)
  }

  return body.data as T
}

/** 专用 fetch 调用（不做 JSON 解析，留给 SSE 使用） */
export function rawFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...init?.headers,
      Authorization: `Bearer ${TOKEN}`,
    },
  })
}
