/** API 统一响应体 */
export interface APIEnvelope<T = unknown> {
  code: number
  message: string
  data?: T
}
