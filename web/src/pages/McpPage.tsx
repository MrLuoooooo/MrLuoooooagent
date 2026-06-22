import { useEffect, useState, useCallback, useRef } from 'react'
import { fetchMcpServers, upsertMcpServer, removeMcpServer, toggleMcpEnabled, importMcpZip } from '../api/mcp'
import type { McpServer, McpImportResult } from '../api/mcp'
import { Server, Plus, Trash2, AlertCircle, ExternalLink, Terminal, Power, Upload, FolderOpen, RefreshCw, Loader2 } from 'lucide-react'

type TabMode = 'sse' | 'stdio' | 'zip' | 'path'
const emptyForm: McpServer = { name: '', transport: 'sse', url: '', command: '', args: [], env: {} }

export default function McpPage() {
  const [servers, setServers] = useState<McpServer[]>([])
  const [enabled, setEnabled] = useState(false)
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [tab, setTab] = useState<TabMode>('sse')
  const [form, setForm] = useState<McpServer>(emptyForm)
  const [argsText, setArgsText] = useState('')
  const [errMsg, setErrMsg] = useState('')
  const [importing, setImporting] = useState(false)
  const [importResult, setImportResult] = useState<McpImportResult | null>(null)
  const [connectedMap, setConnectedMap] = useState<Record<string, boolean>>({})
  const [connectErrors, setConnectErrors] = useState<Record<string, string>>({})
  const fileRef = useRef<HTMLInputElement>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const cfg = await fetchMcpServers()
      setServers(cfg.servers ?? [])
      setEnabled(cfg.enabled ?? false)
      setErrMsg('')
    } catch { setErrMsg('加载 MCP 服务器配置失败') }
    setLoading(false)
  }, [])

  useEffect(() => { load() }, [load])

  const handleSave = async () => {
    if (!form.name.trim()) return
    const srv: McpServer = { ...form, name: form.name.trim() }
    if (srv.transport === 'sse') { srv.command = ''; srv.args = []; srv.env = {} }
    try {
      const result = await upsertMcpServer(srv)
      setShowForm(false); resetForm(); setErrMsg('')
      if (result?.connected) {
        setConnectedMap(m => ({ ...m, [srv.name]: true }))
        delete connectErrors[srv.name]
      } else if (result?.error) {
        setConnectedMap(m => ({ ...m, [srv.name]: false }))
        setConnectErrors(e => ({ ...e, [srv.name]: result.error || '' }))
      }
      load()
    } catch { setErrMsg('保存失败') }
  }

  const handleDelete = async (name: string) => {
    try { await removeMcpServer(name); delete connectedMap[name]; delete connectErrors[name]; setErrMsg(''); load() } catch { setErrMsg('删除失败') }
  }

  const handleToggleEnabled = async () => {
    try { await toggleMcpEnabled(!enabled); setErrMsg(''); load() } catch { setErrMsg('切换失败') }
  }

  const resetForm = () => {
    setForm(emptyForm)
    setArgsText('')
    setImportResult(null)
  }

  const openForm = (mode: TabMode, s?: McpServer) => {
    setTab(mode)
    if (s) {
      setForm({ ...s })
      setArgsText((s.args ?? []).join('\n'))
    } else {
      resetForm()
      setForm({ ...emptyForm, transport: mode === 'sse' ? 'sse' : 'stdio' })
    }
    setShowForm(true)
    setErrMsg('')
  }

  const handleZipImport = async () => {
    const file = fileRef.current?.files?.[0]
    if (!file) return
    setImporting(true)
    setImportResult(null)
    setErrMsg('')
    try {
      const result = await importMcpZip(form.name || file.name.replace(/\.zip$/i, ''), file)
      setImportResult(result)
      if (result.code === 0 && result.connected) {
        setConnectedMap(m => ({ ...m, [result.server?.name || '']: true }))
      } else if (result.error) {
        if (result.server?.name) setConnectErrors(e => ({ ...e, [result.server!.name]: result.error || '' }))
      }
      load()
    } catch (e) {
      setErrMsg('上传失败')
    }
    setImporting(false)
  }

  return (
    <div className="flex flex-1 flex-col min-h-0 min-w-0">
      {errMsg && (
        <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 border-b border-red-200 dark:border-red-800 px-4 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle size={16} /><span>{errMsg}</span>
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-4 min-h-0">
        <div className="mx-auto max-w-4xl py-4 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold flex items-center gap-2"><Server size={18} /> MCP 服务器管理</h2>
            <div className="flex items-center gap-2">
              <button onClick={handleToggleEnabled} className={`flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${enabled ? 'bg-green-500 text-white hover:bg-green-600' : 'bg-gray-300 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-400'}`}>
                <Power size={14} /> {enabled ? '已启用' : '已禁用'}
              </button>
              <button onClick={() => openForm('sse')} className="flex items-center gap-1 rounded-lg bg-purple-500 px-3 py-1.5 text-sm text-white hover:bg-purple-600">
                <Plus size={14} /> 添加服务器
              </button>
            </div>
          </div>

          <p className="text-xs text-gray-400">通过 MCP 协议连接外部工具服务器，上传 ZIP 直接导入项目。</p>

          {showForm && (
            <div className="bg-white dark:bg-gray-900 rounded-xl border border-purple-300 dark:border-purple-700 overflow-hidden">
              <div className="flex border-b border-gray-200 dark:border-gray-700">
                {[
                  { id: 'sse', label: 'SSE 地址', icon: ExternalLink },
                  { id: 'stdio', label: 'Stdio 命令', icon: Terminal },
                  { id: 'zip', label: '上传 ZIP', icon: Upload },
                  { id: 'path', label: '本地路径', icon: FolderOpen },
                ].map(t => (
                  <button key={t.id} onClick={() => setTab(t.id as TabMode)} className={`flex-1 flex items-center justify-center gap-1 py-2 text-xs font-medium transition-colors ${tab === t.id ? 'bg-purple-50 dark:bg-purple-900/20 text-purple-600 border-b-2 border-purple-500' : 'text-gray-500 hover:bg-gray-50 dark:hover:bg-gray-800'}`}>
                    <t.icon size={12} /> {t.label}
                  </button>
                ))}
              </div>
              <div className="p-4 space-y-3">
                <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} placeholder="服务器名称" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800" />
                {tab === 'sse' && (
                  <input value={form.url ?? ''} onChange={e => setForm(f => ({ ...f, transport: 'sse', url: e.target.value }))} placeholder="SSE URL (如 http://localhost:9999/sse)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 font-mono" />
                )}
                {tab === 'stdio' && (
                  <div className="space-y-2">
                    <input value={form.command ?? ''} onChange={e => setForm(f => ({ ...f, transport: 'stdio', command: e.target.value }))} placeholder="命令 (python / node / npx / go)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800" />
                    <textarea value={argsText} onChange={e => setArgsText(e.target.value)} onBlur={() => setForm(f => ({ ...f, args: argsText.split('\n').filter(l => l.trim()) }))} rows={2} placeholder="参数 (一行一个)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 font-mono" />
                  </div>
                )}
                {tab === 'zip' && (
                  <div className="space-y-2">
                    <label className="flex flex-col items-center gap-2 p-6 border-2 border-dashed rounded-lg border-gray-300 dark:border-gray-600 hover:border-purple-400 cursor-pointer transition-colors">
                      <Upload size={24} className="text-gray-400" />
                      <span className="text-sm text-gray-500">点击选择 ZIP 文件或拖拽到此处</span>
                      <input ref={fileRef} type="file" accept=".zip" onChange={() => {}} className="hidden" />
                    </label>
                    {importing && <div className="flex items-center gap-2 text-sm text-purple-500"><Loader2 size={14} className="animate-spin" /> 导入中...</div>}
                    {importResult && (
                      <div className={`rounded-lg p-3 text-sm ${importResult.connected ? 'bg-green-50 dark:bg-green-900/20 text-green-700' : 'bg-red-50 dark:bg-red-900/20 text-red-600'}`}>
                        {importResult.connected ? `✅ 连接成功，加载 ${importResult.tool_count} 个工具` : `❌ ${importResult.error || '连接失败'}`}
                      </div>
                    )}
                  </div>
                )}
                {tab === 'path' && (
                  <div className="space-y-2">
                    <input value={form.command ?? ''} onChange={e => setForm(f => ({ ...f, transport: 'stdio', command: e.target.value }))} placeholder="项目路径 (如 D:\tools\a-share-mcp)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 font-mono" />
                    <p className="text-xs text-gray-400">输入包路径后，系统自动检测 Python / Node / Go 项目类型。</p>
                  </div>
                )}
                <div className="flex gap-2">
                  {tab === 'zip' ? (
                    <button onClick={handleZipImport} disabled={importing} className="rounded-lg bg-purple-500 px-4 py-1.5 text-sm text-white hover:bg-purple-600 disabled:opacity-50">
                      {importing ? '导入中...' : '上传并导入'}
                    </button>
                  ) : (
                    <button onClick={handleSave} className="rounded-lg bg-purple-500 px-4 py-1.5 text-sm text-white hover:bg-purple-600">保存并连接</button>
                  )}
                  <button onClick={() => { setShowForm(false); resetForm() }} className="rounded-lg bg-gray-200 dark:bg-gray-700 px-4 py-1.5 text-sm hover:bg-gray-300">取消</button>
                </div>
              </div>
            </div>
          )}

          {loading && servers.length === 0 && (
            <div className="flex justify-center py-8"><div className="animate-spin h-8 w-8 border-4 border-purple-500 border-t-transparent rounded-full" /></div>
          )}
          {!loading && servers.length === 0 && (
            <div className="text-center text-gray-400 py-12"><Server size={32} className="mx-auto mb-2 opacity-40" /><p>暂无 MCP 服务器</p></div>
          )}
          {servers.map(s => {
            const isConnected = connectedMap[s.name] === true
            const connErr = connectErrors[s.name]
            return (
              <div key={s.name} className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-3">
                <div className="flex items-start gap-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className={`w-2 h-2 rounded-full flex-shrink-0 ${connErr ? 'bg-red-400' : isConnected ? 'bg-green-400' : 'bg-gray-400'}`} />
                      <span className="text-sm font-medium">{s.name}</span>
                      <span className={`text-xs px-1.5 py-0.5 rounded flex items-center gap-1 ${s.transport === 'sse' ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600' : 'bg-purple-100 dark:bg-purple-900/30 text-purple-600'}`}>
                        {s.transport === 'sse' ? <ExternalLink size={10} /> : <Terminal size={10} />} {s.transport}
                      </span>
                    </div>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 font-mono truncate">
                      {s.transport === 'sse' ? s.url : `${s.command} ${(s.args ?? []).join(' ')}`}
                    </p>
                    {connErr && (
                      <details className="mt-1">
                        <summary className="text-xs text-red-500 cursor-pointer">查看错误日志</summary>
                        <pre className="text-xs text-red-400 mt-0.5 whitespace-pre-wrap">{connErr}</pre>
                      </details>
                    )}
                  </div>
                  <div className="flex gap-1 flex-shrink-0">
                    <button onClick={() => { /* reconnect handler */ }} className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-400" title="重新连接"><RefreshCw size={14} /></button>
                    <button onClick={() => openForm(s.transport === 'sse' ? 'sse' : 'stdio', s)} className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-400">编辑</button>
                    <button onClick={() => handleDelete(s.name)} className="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/20 text-red-400" title="删除"><Trash2 size={14} /></button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
