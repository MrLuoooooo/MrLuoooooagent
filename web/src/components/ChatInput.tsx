import { Send, Square, Bot, FolderOpen, ChevronDown, Plus, X } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'
import { fetchWorkspace, setWorkspace as setWorkspaceAPI } from '../api/workspace'
import { fetchModels, switchModel, addCustomModel, removeCustomModel } from '../api/models'
import type { ModelItem, CustomModelForm } from '../types/models'

interface ChatInputProps {
  onSend: (text: string, agent?: boolean) => void
  onCancel: () => void
  disabled?: boolean
  streaming?: boolean
  placeholder?: string
}

export default function ChatInput({
  onSend,
  onCancel,
  disabled,
  streaming,
  placeholder = '输入消息…',
}: ChatInputProps) {
  const [text, setText] = useState('')
  const [agentMode, setAgentMode] = useState(false)
  const [wsInput, setWsInput] = useState('')
  const [wsLoading, setWsLoading] = useState(false)
  const [wsError, setWsError] = useState('')
  const [models, setModels] = useState<ModelItem[]>([])
  const [activeModel, setActiveModel] = useState('')
  const [showCustomModel, setShowCustomModel] = useState(false)
  const [customForm, setCustomForm] = useState<CustomModelForm>({
    name: '', provider: 'openai', api_key: '', base_url: '', chat_model: '', embedding_model: ''
  })
  const [customError, setCustomError] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // 自动调整高度
  useEffect(() => {
    const el = textareaRef.current
    if (el) {
      el.style.height = 'auto'
      el.style.height = Math.min(el.scrollHeight, 200) + 'px'
    }
  }, [text])

  // 加载工作目录和模型列表
  useEffect(() => {
    fetchWorkspace().then((res) => {
      setWsInput(res.path)
    }).catch(() => {})
    fetchModels().then((list) => {
      setModels(list)
      const active = list.find((m) => m.active)
      if (active) setActiveModel(active.name)
    }).catch(() => {})
  }, [])

  const handleSend = () => {
    const trimmed = text.trim()
    if (!trimmed || disabled || streaming) return
    onSend(trimmed, agentMode)
    setText('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleSetWorkspace = () => {
    const trimmed = wsInput.trim()
    console.log('[ChatInput] handleSetWorkspace called, input:', trimmed)
    if (!trimmed) return
    setWsError('')
    setWsLoading(true)
    setWorkspaceAPI(trimmed).then((res) => {
      console.log('[ChatInput] workspace set OK:', res)
      setWsInput(res.path)
      setWsLoading(false)
    }).catch((err) => {
      console.error('[ChatInput] workspace set FAIL:', err)
      setWsError(err.message || '设置失败')
      setWsLoading(false)
    })
  }

  const handleModelChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const name = e.target.value
    if (!name) return
    switchModel(name).then(() => {
      return fetchModels()
    }).then((list) => {
      setModels(list)
      const active = list.find((m) => m.active)
      if (active) setActiveModel(active.name)
    }).catch(() => {})
  }

  const handleAddCustomModel = () => {
    setCustomError('')
    addCustomModel(customForm).then(() => {
      return fetchModels()
    }).then((list) => {
      setModels(list)
      setShowCustomModel(false)
      setCustomForm({ name: '', provider: 'openai', api_key: '', base_url: '', chat_model: '', embedding_model: '' })
    }).catch((err) => {
      setCustomError(err.message || '添加失败')
    })
  }

  const handleRemoveCustomModel = (name: string) => {
    if (!confirm(`确定删除自定义模型 "${name}"？`)) return
    removeCustomModel(name).then(() => {
      return fetchModels()
    }).then((list) => {
      setModels(list)
      const active = list.find((m) => m.active)
      if (active) setActiveModel(active ? active.name : '')
    }).catch(() => {})
  }

  return (
    <div className="border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900">
      {/* 底部工具栏：工作目录 + 模型选择 */}
      <div className="mx-auto max-w-3xl px-4 pt-2 pb-1">
        <div className="flex items-center gap-3">
        {/* 工作目录 */}
        <div className="flex items-center gap-1 flex-1 min-w-0">
          <FolderOpen size={14} className="text-gray-400 flex-shrink-0" />
          <input
            value={wsInput}
            onChange={(e) => setWsInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') handleSetWorkspace() }}
            placeholder="工作目录路径…"
            className="flex-1 min-w-0 rounded border border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 px-2 py-1 text-xs outline-none focus:border-blue-500"
          />
          <button
            onClick={handleSetWorkspace}
            disabled={wsLoading}
            className="flex-shrink-0 rounded bg-blue-500 px-2 py-1 text-xs text-white hover:bg-blue-600 transition-colors disabled:opacity-50 cursor-pointer"
          >
            {wsLoading ? '设置中…' : '设置目录'}
          </button>
        </div>

        {/* 模型选择 */}
        <div className="flex items-center gap-1 flex-shrink-0">
          <ChevronDown size={14} className="text-gray-400" />
          <select
            value={activeModel}
            onChange={handleModelChange}
            className="rounded border border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 px-2 py-1 text-xs outline-none focus:border-blue-500"
          >
            {models.length === 0 && <option value="">加载中…</option>}
            {models.map((m) => (
              <option key={m.name} value={m.name}>
                {m.name}{m.is_custom ? ' (自定义)' : ''}
              </option>
            ))}
          </select>
          {/* 自定义模型 */}
          <button
            onClick={() => setShowCustomModel(!showCustomModel)}
            className="flex-shrink-0 rounded border border-gray-300 dark:border-gray-600 px-1.5 py-1 text-xs text-gray-500 hover:text-blue-500 hover:border-blue-400 transition-colors"
            title="添加自定义模型"
          >
            <Plus size={14} />
          </button>
        </div>
        </div>
        {wsError && (
          <div className="text-xs text-red-500 mt-1">{wsError}</div>
        )}
        {/* 自定义模型列表 */}
        {models.filter(m => m.is_custom).length > 0 && (
          <div className="flex flex-wrap gap-1 mt-1">
            {models.filter(m => m.is_custom).map((m) => (
              <span key={m.name} className="inline-flex items-center gap-0.5 rounded bg-purple-100 dark:bg-purple-900/30 px-1.5 py-0.5 text-xs text-purple-700 dark:text-purple-300">
                {m.name}
                <button onClick={() => handleRemoveCustomModel(m.name)} className="hover:text-red-500" title="删除">
                  <X size={10} />
                </button>
              </span>
            ))}
          </div>
        )}
        {/* 自定义模型弹窗 */}
        {showCustomModel && (
          <div className="mt-2 p-3 rounded-lg border border-purple-200 dark:border-purple-700 bg-purple-50 dark:bg-purple-900/20">
            <div className="grid grid-cols-2 gap-2 mb-2">
              <input
                value={customForm.name}
                onChange={(e) => setCustomForm({ ...customForm, name: e.target.value })}
                placeholder="模型名称"
                className="rounded border px-2 py-1 text-xs outline-none focus:border-purple-500 bg-white dark:bg-gray-800"
              />
              <input
                value={customForm.base_url}
                onChange={(e) => setCustomForm({ ...customForm, base_url: e.target.value })}
                placeholder="API地址 (如 https://api.openai.com/v1)"
                className="rounded border px-2 py-1 text-xs outline-none focus:border-purple-500 bg-white dark:bg-gray-800"
              />
              <input
                value={customForm.api_key}
                onChange={(e) => setCustomForm({ ...customForm, api_key: e.target.value })}
                placeholder="API Key"
                type="password"
                className="rounded border px-2 py-1 text-xs outline-none focus:border-purple-500 bg-white dark:bg-gray-800"
              />
              <input
                value={customForm.chat_model}
                onChange={(e) => setCustomForm({ ...customForm, chat_model: e.target.value })}
                placeholder="Chat模型名 (如 gpt-4o)"
                className="rounded border px-2 py-1 text-xs outline-none focus:border-purple-500 bg-white dark:bg-gray-800"
              />
            </div>
            {customError && <p className="text-xs text-red-500 mb-2">{customError}</p>}
            <div className="flex gap-2">
              <button
                onClick={handleAddCustomModel}
                disabled={!customForm.name || !customForm.base_url}
                className="rounded bg-purple-500 px-3 py-1 text-xs text-white hover:bg-purple-600 disabled:opacity-50"
              >
                添加
              </button>
              <button
                onClick={() => setShowCustomModel(false)}
                className="rounded bg-gray-200 dark:bg-gray-700 px-3 py-1 text-xs hover:bg-gray-300 dark:hover:bg-gray-600"
              >
                取消
              </button>
            </div>
          </div>
        )}
      </div>

      {/* 输入区域 */}
      <div className="mx-auto max-w-3xl flex items-end gap-2 p-4 pt-2">
        {/* Agent 模式切换 */}
        <button
          onClick={() => setAgentMode(!agentMode)}
          disabled={disabled || streaming}
          className={`flex-shrink-0 rounded-xl px-3 py-3 text-sm font-medium transition-colors ${
            agentMode
              ? 'bg-purple-500 text-white hover:bg-purple-600'
              : 'bg-gray-100 dark:bg-gray-800 text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-700'
          } disabled:opacity-50`}
          title={agentMode ? 'Agent 模式（可调用工具）' : 'RAG 模式'}
        >
          <Bot size={16} />
        </button>

        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={agentMode ? 'Agent 模式 — AI 可搜索/查时间…' : placeholder}
          rows={1}
          disabled={disabled}
          className="flex-1 resize-none rounded-xl border border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 px-4 py-3 text-sm outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
        />

        {streaming ? (
          <button
            onClick={onCancel}
            className="flex-shrink-0 rounded-xl bg-red-500 px-4 py-3 text-white hover:bg-red-600 transition-colors"
            title="停止生成"
          >
            <Square size={16} />
          </button>
        ) : (
          <button
            onClick={handleSend}
            disabled={!text.trim() || disabled}
            className="flex-shrink-0 rounded-xl bg-blue-500 px-4 py-3 text-white hover:bg-blue-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            title="发送"
          >
            <Send size={16} />
          </button>
        )}
      </div>
    </div>
  )
}
