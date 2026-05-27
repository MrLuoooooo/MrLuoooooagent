export interface APIEnvelope<T = unknown> {
  code: number
  message: string
  data?: T
}
