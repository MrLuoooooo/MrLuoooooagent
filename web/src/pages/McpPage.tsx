import { useEffect, useState, useCallback, useRef } from 'react'
import { fetchMcpServers, removeMcpServer, toggleMcpEnabled, importMcpZip } from '../api/mcp'
import type { McpServer } from '../api/mcp'
import { Server, Plus, Trash2, AlertCircle, ExternalLink, Terminal, Power, FolderOpen, RefreshCw, Loader2, FileArchive } from 'lucide-react'
import JSZip from 'jszip'

const emptyForm: McpServer = { name: '', transport: 'sse', url: '', command: '', args: [], env: {} }

export default function McpPage() {
  const [servers, setServers] = useState<McpServer[]>([])
  const [enabled, setEnabled] = useState(false)
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState<McpServer>(emptyForm)
  const [errMsg, setErrMsg] = useState('')
  const [importing, setImporting] = useState(false)
  const [importResult, setImportResult] = useState<string | null>(null)
  const [connectedMap, setConnectedMap] = useState<Record<string, boolean>>({})
  const [connectErrors, setConnectErrors] = useState<Record<string, string>>({})
  const folderInputRef = useRef<HTMLInputElement>(null)
  const zipInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const el = folderInputRef.current
    if (el) { el.setAttribute('webkitdirectory', ''); el.setAttribute('directory', '') }
  }, [showForm])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const cfg = await fetchMcpServers()
      setServers(cfg.servers ?? [])
      setEnabled(cfg.enabled ?? false)
      setErrMsg('')
    } catch (e: any) { setErrMsg('加载失败: ' + (e?.message || e?.toString?.() || '网络错误')) }
    setLoading(false)
  }, [])

  useEffect(() => { load() }, [load])

  const openForm = () => {
    setForm(emptyForm)
    setImportResult(null)
    setShowForm(true)
    setErrMsg('')
  }

  const handleDelete = async (name: string) => {
    try { await removeMcpServer(name); delete connectedMap[name]; delete connectErrors[name]; load() } catch { setErrMsg('删除失败') }
  }

  const handleToggleEnabled = async () => {
    try { await toggleMcpEnabled(!enabled); load() } catch { setErrMsg('切换失败') }
  }

  const handleFileUpload = async (input: File | FileList, isZip: boolean) => {
    setImporting(true)
    setImportResult(null)
    setErrMsg('')
    try {
      let file: File
      if (isZip) {
        const f = input as File
        if (!f.name.toLowerCase().endsWith('.zip')) { setErrMsg('请选择 .zip 文件'); setImporting(false); return }
        file = f
      } else {
        const fl = input as FileList
        if (!fl || fl.length === 0) { setErrMsg('未选择任何文件'); setImporting(false); return }
        if (!(fl[0] as any).webkitRelativePath) { setErrMsg('文件夹导入需要 Chrome/Edge 等浏览器支持'); setImporting(false); return }
        const zip = new JSZip()
        for (let i = 0; i < fl.length; i++) {
          const f = fl[i]
          const relPath = (f as any).webkitRelativePath || f.name
          if (!relPath) continue
          zip.file(relPath, f)
        }
        const name = form.name || 'mcp-project'
        const blob = await zip.generateAsync({ type: 'blob' })
        file = new File([blob], name + '.zip', { type: 'application/zip' })
      }
      const result = await importMcpZip(form.name || file.name.replace(/\.zip$/i, ''), file)
      if (result.code !== 0 || !result.connected) {
        setImportResult('❌ ' + (result.error || result.message || '连接失败'))
        if (result.server?.name) setConnectErrors(e => ({ ...e, [result.server!.name]: result.error || '' }))
      } else {
        setImportResult('✅ 连接成功，加载 ' + result.tool_count + ' 个工具')
        if (result.server?.name) setConnectedMap(m => ({ ...m, [result.server!.name as string]: true }))
      }
      load()
    } catch (e: any) {
      setImportResult('❌ ' + (e?.message || '上传失败'))
    }
    setImporting(false)
    if (isZip && zipInputRef.current) zipInputRef.current.value = ''
    if (!isZip && folderInputRef.current) folderInputRef.current.value = ''
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
              <button onClick={openForm} className="flex items-center gap-1 rounded-lg bg-purple-500 px-3 py-1.5 text-sm text-white hover:bg-purple-600">
                <Plus size={14} /> 添加服务器
              </button>
            </div>
          </div>
          <p className="text-xs text-gray-400">通过 MCP 协议连接外部工具服务器。上传项目 zip 包或本地文件夹后自动识别 MCP server。</p>

          {showForm && (
            <div className="bg-white dark:bg-gray-900 rounded-xl border border-purple-300 dark:border-purple-700 overflow-hidden">
              <div className="p-4 space-y-3">
                <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} placeholder="项目名称" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800" />
                <div className="space-y-3">
                  <div className="grid grid-cols-2 gap-3">
                    <label className="relative flex flex-col items-center gap-2 p-4 border-2 border-dashed rounded-lg border-gray-300 dark:border-gray-600 hover:border-purple-400 cursor-pointer transition-colors">
                      <FileArchive size={22} className="text-gray-400" />
                      <span className="text-xs text-gray-500">ZIP 压缩包</span>
                      <input ref={zipInputRef} type="file" accept=".zip" onChange={e => { const f = e.target.files?.[0]; if (f) handleFileUpload(f, true) }} className="hidden" />
                    </label>
                    <label className="relative flex flex-col items-center gap-2 p-4 border-2 border-dashed rounded-lg border-gray-300 dark:border-gray-600 hover:border-purple-400 cursor-pointer transition-colors">
                      <FolderOpen size={22} className="text-gray-400" />
                      <span className="text-xs text-gray-500">本地文件夹</span>
                      <input type="file" ref={folderInputRef} onChange={e => { const fs = e.target.files; if (fs && fs.length > 0) handleFileUpload(fs, false) }} className="absolute inset-0 opacity-0" />
                    </label>
                  </div>
                  {importing && <div className="flex items-center gap-2 text-sm text-purple-500"><Loader2 size={14} className="animate-spin" /> 正在导入...</div>}
                  {importResult && (
                    <div className={`rounded-lg p-3 text-sm ${importResult.startsWith('✅') ? 'bg-green-50 dark:bg-green-900/20 text-green-700' : 'bg-red-50 dark:bg-red-900/20 text-red-600'}`}>
                      {importResult}
                    </div>
                  )}
                </div>
                <div className="flex gap-2">
                  <button onClick={() => { setShowForm(false); setForm(emptyForm); setImportResult(null) }} className="rounded-lg bg-gray-200 dark:bg-gray-700 px-4 py-1.5 text-sm hover:bg-gray-300">取消</button>
                </div>
              </div>
            </div>
          )}

          {importing && !showForm && (
            <div className="flex items-center gap-2 text-sm text-purple-500 justify-center py-2"><Loader2 size={14} className="animate-spin" /> 正在导入...</div>
          )}
          {importResult && !showForm && (
            <div className={`rounded-lg p-3 text-sm text-center ${importResult.startsWith('✅') ? 'bg-green-50 dark:bg-green-900/20 text-green-700' : 'bg-red-50 dark:bg-red-900/20 text-red-600'}`}>
              {importResult}
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
                    <button onClick={() => {}} className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-400" title="重新连接"><RefreshCw size={14} /></button>
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
