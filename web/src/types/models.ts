export interface ModelItem {
  name: string
  provider: string
  is_local: boolean
  is_custom: boolean
  active: boolean
}

export interface CustomModelForm {
  name: string
  provider: string
  api_key: string
  base_url: string
  chat_model: string
  embedding_model: string
}

export interface SkillItem {
  name: string
  prompt: string
  enabled: boolean
}
