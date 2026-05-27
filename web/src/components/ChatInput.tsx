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
  const [wsSuccess, setWsSuccess] = useState(false)
  const [models, setModels] = useState<ModelItem[]>([])
  const [modelsLoaded, setModelsLoaded] = useState(false)
  const [activeModel, setActiveModel] = useState('')
  const [showCustomModel, setShowCustomModel] = useState(false)
  const [customForm, setCustomForm] = useState<CustomModelForm>({
    name: '', provider: 'openai', api_key: '', base_url: '', chat_model: '', embedding_model: ''
  })
  const [customError, setCustomError] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    const el = textareaRef.current
    if (el) {
      el.style.height = 'auto'
      el.style.height = Math.min(el.scrollHeight, 200) + 'px'
    }
  }, [text])

  useEffect(() => {
    fetchWorkspace().then((res) => {
      setWsInput(res.path)
    }).catch(() => {})
    fetchModels().then((list) => {
      const safe = list ?? []
      setModels(safe)
      setModelsLoaded(true)
      const active = safe.find((m) => m.active)
      if (active) setActiveModel(active.name)
    }).catch(() => { setModelsLoaded(true) })
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
    if (!trimmed) return
    setWsSuccess(false)
    setWsLoading(true)
    setWorkspaceAPI(trimmed).then((res) => {
      setWsInput(res.path)
      setWsLoading(false)
      setWsSuccess(true)
      setTimeout(() => setWsSuccess(false), 2000)
    }).catch(() => {
      setWsLoading(false)
    })
  }

  const handleModelChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const name = e.target.value
    if (!name) return
    switchModel(name).then(() => {
      return fetchModels()
    }).then((list) => {
      const safe = list ?? []
      setModels(safe)
      const active = safe.find((m) => m.active)
      if (active) setActiveModel(active.name)
    }).catch(() => {})
  }

  const handleAddCustomModel = () => {
    setCustomError('')
    addCustomModel(customForm).then(() => {
      return fetchModels()
    }).then((list) => {
      setModels(list ?? [])
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
      const safe = list ?? []
      setModels(safe)
      const active = safe.find((m) => m.active)
      setActiveModel(active ? active.name : '')
    }).catch(() => {})
  }

  return (
    <div className="border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-950">
      {/* 底部工具栏 */}
      <div className="mx-auto max-w-3xl px-4 pt-2 pb-1">
        <div className="flex items-center gap-2">
          {/* 工作目录 */}
          <div className="flex items-center gap-1 flex-1 min-w-0">
            <FolderOpen size={13} className="text-gray-400 flex-shrink-0" />
            <input
              value={wsInput}
              onChange={(e) => setWsInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleSetWorkspace() }}
              placeholder="工作目录路径…"
              className="flex-1 min-w-0 rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 px-2.5 py-1.5 text-xs outline-none focus:border-blue-400 dark:focus:border-blue-600 focus:ring-1 focus:ring-blue-400/20 dark:focus:ring-blue-600/20 transition-all placeholder:text-gray-400"
            />
            <button
              onClick={handleSetWorkspace}
              disabled={wsLoading}
              className={`flex-shrink-0 rounded-lg px-2.5 py-1.5 text-xs text-white font-medium transition-colors disabled:opacity-50 cursor-pointer ${
                wsSuccess
                  ? 'bg-green-500'
                  : wsLoading
                    ? 'bg-blue-400'
                    : 'bg-blue-500 hover:bg-blue-600'
              }`}
            >
              {wsLoading ? '设置中…' : wsSuccess ? '✓ 已设置' : '设置'}
            </button>
          </div>

          <div className="flex items-center gap-1 flex-shrink-0">
            <div className="relative">
              <select
                value={activeModel}
                onChange={handleModelChange}
                className="appearance-none rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 pl-2.5 pr-7 py-1.5 text-xs outline-none focus:border-blue-400 dark:focus:border-blue-600 focus:ring-1 focus:ring-blue-400/20 cursor-pointer transition-all"
              >
                {!modelsLoaded && <option value="">加载中…</option>}
                {modelsLoaded && models.length === 0 && <option value="">暂无可用模型</option>}
                {models.map((m) => (
                  <option key={m.name} value={m.name}>
                    {m.name}{m.is_custom ? ' (自定义)' : ''}
                  </option>
                ))}
              </select>
              <ChevronDown size={12} className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
            </div>
            <button
              onClick={() => setShowCustomModel(!showCustomModel)}
              className="flex-shrink-0 rounded-lg border border-gray-200 dark:border-gray-700 px-1.5 py-1.5 text-gray-500 hover:text-blue-500 hover:border-blue-300 dark:hover:border-blue-600 transition-all"
              title="添加自定义模型"
            >
              <Plus size={13} />
            </button>
          </div>
        </div>


        {models.filter(m => m.is_custom).length > 0 && (
          <div className="flex flex-wrap gap-1 mt-1">
            {models.filter(m => m.is_custom).map((m) => (
              <span key={m.name} className="inline-flex items-center gap-1 rounded-full bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800 px-2 py-0.5 text-[11px] text-purple-700 dark:text-purple-300">
                {m.name}
                <button onClick={() => handleRemoveCustomModel(m.name)} className="hover:text-red-500 transition-colors" title="删除">
                  <X size={10} />
                </button>
              </span>
            ))}
          </div>
        )}

        {showCustomModel && (
          <div className="mt-2 p-4 rounded-xl border border-purple-200 dark:border-purple-700 bg-purple-50 dark:bg-purple-900/20 shadow-sm">
            <h4 className="text-xs font-semibold text-purple-700 dark:text-purple-300 mb-2">添加自定义模型</h4>
            <div className="grid grid-cols-2 gap-2 mb-2">
              <input
                value={customForm.name}
                onChange={(e) => setCustomForm({ ...customForm, name: e.target.value })}
                placeholder="模型名称"
                className="rounded-lg border border-gray-200 dark:border-gray-600 px-2.5 py-1.5 text-xs outline-none focus:border-purple-400 focus:ring-1 focus:ring-purple-400/20 bg-white dark:bg-gray-800 transition-all"
              />
              <input
                value={customForm.base_url}
                onChange={(e) => setCustomForm({ ...customForm, base_url: e.target.value })}
                placeholder="API 地址"
                className="rounded-lg border border-gray-200 dark:border-gray-600 px-2.5 py-1.5 text-xs outline-none focus:border-purple-400 focus:ring-1 focus:ring-purple-400/20 bg-white dark:bg-gray-800 transition-all"
              />
              <input
                value={customForm.api_key}
                onChange={(e) => setCustomForm({ ...customForm, api_key: e.target.value })}
                placeholder="API Key"
                type="password"
                className="rounded-lg border border-gray-200 dark:border-gray-600 px-2.5 py-1.5 text-xs outline-none focus:border-purple-400 focus:ring-1 focus:ring-purple-400/20 bg-white dark:bg-gray-800 transition-all"
              />
              <input
                value={customForm.chat_model}
                onChange={(e) => setCustomForm({ ...customForm, chat_model: e.target.value })}
                placeholder="Chat 模型名"
                className="rounded-lg border border-gray-200 dark:border-gray-600 px-2.5 py-1.5 text-xs outline-none focus:border-purple-400 focus:ring-1 focus:ring-purple-400/20 bg-white dark:bg-gray-800 transition-all"
              />
            </div>
            {customError && <p className="text-xs text-red-500 mb-2">{customError}</p>}
            <div className="flex gap-2">
              <button
                onClick={handleAddCustomModel}
                disabled={!customForm.name || !customForm.base_url}
                className="rounded-lg bg-purple-500 px-4 py-1.5 text-xs text-white hover:bg-purple-600 disabled:opacity-50 transition-colors font-medium"
              >
                添加
              </button>
              <button
                onClick={() => setShowCustomModel(false)}
                className="rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 px-4 py-1.5 text-xs text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
              >
                取消
              </button>
            </div>
          </div>
        )}
      </div>

      {/* 输入区域 */}
      <div className="mx-auto max-w-3xl flex items-end gap-2 px-4 pb-3 pt-1.5">
        {/* Agent 模式切换 */}
        <button
          onClick={() => setAgentMode(!agentMode)}
          disabled={disabled || streaming}
          className={`flex-shrink-0 rounded-xl px-3 py-3 text-sm font-medium transition-all duration-150 ${
            agentMode
              ? 'bg-gradient-to-br from-purple-500 to-purple-600 text-white shadow-sm shadow-purple-200 dark:shadow-purple-900/30 hover:from-purple-600 hover:to-purple-700'
              : 'bg-gray-100 dark:bg-gray-800 text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-700 hover:text-gray-700 dark:hover:text-gray-300'
          } disabled:opacity-50`}
          title={agentMode ? 'Agent 模式（可调用工具）' : 'RAG 模式'}
        >
          <Bot size={16} />
        </button>

        {/* 输入框 */}
        <div className="flex-1 relative">
          <textarea
            ref={textareaRef}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={agentMode ? 'Agent 模式 — AI 可搜索文件、查询时间等…' : placeholder}
            rows={1}
            disabled={disabled}
            className="w-full resize-none rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 pl-4 pr-4 py-3 text-sm outline-none focus:border-blue-400 dark:focus:border-blue-600 focus:ring-1 focus:ring-blue-400/20 dark:focus:ring-blue-600/20 disabled:opacity-50 transition-all placeholder:text-gray-400"
          />
        </div>

        {streaming ? (
          <button
            onClick={onCancel}
            className="flex-shrink-0 rounded-xl bg-red-500 px-4 py-3 text-white hover:bg-red-600 transition-colors shadow-sm shadow-red-200 dark:shadow-red-900/30"
            title="停止生成"
          >
            <Square size={16} />
          </button>
        ) : (
          <button
            onClick={handleSend}
            disabled={!text.trim() || disabled}
            className={`flex-shrink-0 rounded-xl px-4 py-3 text-white transition-all duration-150 ${
              text.trim() && !disabled
                ? 'bg-gradient-to-br from-blue-500 to-blue-600 shadow-sm shadow-blue-200 dark:shadow-blue-900/30 hover:from-blue-600 hover:to-blue-700 hover:shadow-md'
                : 'bg-gray-300 dark:bg-gray-700 cursor-not-allowed'
            }`}
            title="发送"
          >
            <Send size={16} />
          </button>
        )}
      </div>
    </div>
  )
}
