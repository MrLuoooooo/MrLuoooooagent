import { useEffect, useState, useCallback } from 'react'
import { fetchMcpServers, upsertMcpServer, removeMcpServer, toggleMcpEnabled } from '../api/mcp'
import type { McpServer } from '../api/mcp'
import { Server, Plus, Trash2, AlertCircle, ExternalLink, Terminal, Power } from 'lucide-react'

const emptyForm: McpServer = { name: '', transport: 'sse', url: '', command: '', args: [], env: {} }

export default function McpPage() {
  const [servers, setServers] = useState<McpServer[]>([])
  const [enabled, setEnabled] = useState(false)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<McpServer | null>(null)
  const [form, setForm] = useState<McpServer>(emptyForm)
  const [errMsg, setErrMsg] = useState('')
  const [argsText, setArgsText] = useState('')
  const [envText, setEnvText] = useState('')

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
      await upsertMcpServer(srv)
      setEditing(null); setForm(emptyForm); setArgsText(''); setEnvText(''); setErrMsg('')
      load()
    } catch { setErrMsg('保存失败') }
  }

  const handleDelete = async (name: string) => {
    try { await removeMcpServer(name); setErrMsg(''); load() } catch { setErrMsg('删除失败') }
  }

  const handleToggleEnabled = async () => {
    try { await toggleMcpEnabled(!enabled); setErrMsg(''); load() } catch { setErrMsg('切换失败') }
  }

  const startEdit = (s?: McpServer) => {
    if (s) {
      setEditing(s)
      setForm({ ...s })
      setArgsText((s.args ?? []).join('\n'))
      const envLines = []
      if (s.env) for (const [k, v] of Object.entries(s.env)) envLines.push(`${k}=${v}`)
      setEnvText(envLines.join('\n'))
    } else {
      setEditing(emptyForm)
      setForm(emptyForm)
      setArgsText('')
      setEnvText('')
    }
    setErrMsg('')
  }

  const parseForm = () => {
    setForm(f => ({
      ...f,
      args: f.transport === 'stdio' ? argsText.split('\n').filter(l => l.trim()) : [],
      env: f.transport === 'stdio' ? envText.split('\n').filter(l => l.includes('=')).reduce((acc, l) => {
        const [k, ...v] = l.split('=')
        if (k) acc[k] = v.join('=')
        return acc
      }, {} as Record<string, string>) : {},
    }))
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
              <button onClick={() => startEdit()} className="flex items-center gap-1 rounded-lg bg-purple-500 px-3 py-1.5 text-sm text-white hover:bg-purple-600">
                <Plus size={14} /> 添加服务器
              </button>
            </div>
          </div>

          <p className="text-xs text-gray-400">通过 MCP 协议连接外部工具服务器，扩展 Agent 能力。</p>

          {editing !== null && (
            <div className="bg-white dark:bg-gray-900 rounded-xl border border-purple-300 dark:border-purple-700 p-4 space-y-3">
              <h3 className="text-sm font-medium">{editing.name ? '编辑服务器' : '添加服务器'}</h3>
              <div className="grid grid-cols-2 gap-3">
                <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} placeholder="名称 (如 a-share-mcp)" className="rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 w-full" />
                <select value={form.transport} onChange={e => {
                  const t = e.target.value as 'sse' | 'stdio'
                  setForm(f => ({ ...f, transport: t }))
                }} className="rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800">
                  <option value="sse">SSE (远程)</option>
                  <option value="stdio">Stdio (本地)</option>
                </select>
              </div>
              {form.transport === 'sse' ? (
                <input value={form.url ?? ''} onChange={e => setForm(f => ({ ...f, url: e.target.value }))} placeholder="SSE URL (如 http://localhost:9999/sse)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800" />
              ) : (
                <div className="space-y-2">
                  <input value={form.command ?? ''} onChange={e => setForm(f => ({ ...f, command: e.target.value }))} placeholder="命令 (如 python 或 npx)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800" />
                  <textarea value={argsText} onChange={e => setArgsText(e.target.value)} onBlur={parseForm} rows={2} placeholder="参数 (一行一个)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 font-mono" />
                  <textarea value={envText} onChange={e => setEnvText(e.target.value)} onBlur={parseForm} rows={2} placeholder="环境变量 (KEY=VALUE，一行一个)" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 font-mono resize-y" />
                </div>
              )}
              <div className="flex gap-2">
                <button onClick={handleSave} className="rounded-lg bg-purple-500 px-4 py-1.5 text-sm text-white hover:bg-purple-600">保存</button>
                <button onClick={() => setEditing(null)} className="rounded-lg bg-gray-200 dark:bg-gray-700 px-4 py-1.5 text-sm hover:bg-gray-300">取消</button>
              </div>
            </div>
          )}

          {loading && servers.length === 0 && (
            <div className="flex justify-center py-8"><div className="animate-spin h-8 w-8 border-4 border-purple-500 border-t-transparent rounded-full" /></div>
          )}
          {!loading && servers.length === 0 && (
            <div className="text-center text-gray-400 py-12"><Server size={32} className="mx-auto mb-2 opacity-40" /><p>暂无 MCP 服务器</p></div>
          )}
          {servers.map(s => (
            <div key={s.name} className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-3">
              <div className="flex items-start gap-3">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{s.name}</span>
                    <span className={`text-xs px-1.5 py-0.5 rounded flex items-center gap-1 ${s.transport === 'sse' ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400' : 'bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400'}`}>
                      {s.transport === 'sse' ? <ExternalLink size={10} /> : <Terminal size={10} />}
                      {s.transport}
                    </span>
                  </div>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 font-mono truncate">
                    {s.transport === 'sse' ? s.url : `${s.command} ${(s.args ?? []).join(' ')}`}
                  </p>
                </div>
                <div className="flex gap-1 flex-shrink-0">
                  <button onClick={() => startEdit(s)} className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-400 text-xs">编辑</button>
                  <button onClick={() => handleDelete(s.name)} className="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/20 text-red-400" title="删除"><Trash2 size={14} /></button>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
