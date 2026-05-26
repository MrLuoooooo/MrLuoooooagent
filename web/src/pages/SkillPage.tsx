import { useEffect, useState, useCallback } from 'react'
import { fetchSkills, upsertSkill, removeSkill } from '../api/skills'
import type { SkillItem } from '../types/models'
import { Puzzle, Plus, Trash2, Check, X, Edit2, AlertCircle } from 'lucide-react'

export default function SkillPage() {
  const [skills, setSkills] = useState<SkillItem[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<SkillItem | null>(null)
  const [form, setForm] = useState({ name: '', prompt: '', enabled: true })
  const [errMsg, setErrMsg] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try { const list = await fetchSkills(); setSkills(list ?? []); setErrMsg('') } catch { setErrMsg('加载技能列表失败') }
    setLoading(false)
  }, [])

  useEffect(() => { load() }, [load])

  const handleSave = async () => {
    if (!form.name.trim() || !form.prompt.trim()) return
    try {
      await upsertSkill({ name: form.name.trim(), prompt: form.prompt.trim(), enabled: form.enabled })
      setEditing(null)
      setForm({ name: '', prompt: '', enabled: true })
      setErrMsg('')
      load()
    } catch { setErrMsg('保存失败，请重试') }
  }

  const handleToggle = async (s: SkillItem) => {
    try { await upsertSkill({ ...s, enabled: !s.enabled }); setErrMsg(''); load() } catch { setErrMsg('操作失败') }
  }

  const handleDelete = async (name: string) => {
    try { await removeSkill(name); setErrMsg(''); load() } catch { setErrMsg('删除失败') }
  }

  const startEdit = (s: SkillItem) => {
    setEditing(s)
    setForm({ name: s.name, prompt: s.prompt, enabled: s.enabled })
    setErrMsg('')
  }

  return (
    <div className="flex flex-1 flex-col min-h-0 min-w-0">
      {errMsg && (
        <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 border-b border-red-200 dark:border-red-800 px-4 py-2 text-sm text-red-600 dark:text-red-400">
          <AlertCircle size={16} /><span>{errMsg}</span>
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-4 min-h-0">
        <div className="mx-auto max-w-3xl py-4 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold flex items-center gap-2"><Puzzle size={18} /> 自定义技能</h2>
            <button onClick={() => { setEditing({ name: '', prompt: '', enabled: true }); setForm({ name: '', prompt: '', enabled: true }); setErrMsg('') }} className="flex items-center gap-1 rounded-lg bg-purple-500 px-3 py-1.5 text-sm text-white hover:bg-purple-600">
              <Plus size={14} /> 新建技能
            </button>
          </div>

          <p className="text-xs text-gray-400">技能是自定义提示词，启用后自动注入到 Agent 的系统 prompt 中。</p>

          {editing !== null && (
            <div className="bg-white dark:bg-gray-900 rounded-xl border border-purple-300 dark:border-purple-700 p-4 space-y-3">
              <h3 className="text-sm font-medium">{editing.name ? '编辑技能' : '新建技能'}</h3>
              <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="技能名称" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800" />
              <textarea value={form.prompt} onChange={e => setForm({ ...form, prompt: e.target.value })} rows={4} placeholder="技能提示词" className="w-full rounded-lg border px-3 py-2 text-sm bg-gray-50 dark:bg-gray-800 resize-y" />
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.enabled} onChange={e => setForm({ ...form, enabled: e.target.checked })} className="rounded" /> 启用</label>
              <div className="flex gap-2">
                <button onClick={handleSave} className="rounded-lg bg-purple-500 px-4 py-1.5 text-sm text-white hover:bg-purple-600">保存</button>
                <button onClick={() => setEditing(null)} className="rounded-lg bg-gray-200 dark:bg-gray-700 px-4 py-1.5 text-sm hover:bg-gray-300">取消</button>
              </div>
            </div>
          )}

          {loading && skills.length === 0 && (
            <div className="flex justify-center py-8"><div className="animate-spin h-8 w-8 border-4 border-purple-500 border-t-transparent rounded-full" /></div>
          )}
          {!loading && skills.length === 0 && (
            <div className="text-center text-gray-400 py-12"><Puzzle size={32} className="mx-auto mb-2 opacity-40" /><p>暂无自定义技能</p></div>
          )}
          {skills.map(s => (
            <div key={s.name} className={`bg-white dark:bg-gray-900 rounded-lg border px-4 py-3 flex items-start gap-3 ${s.enabled ? 'border-green-200 dark:border-green-800' : 'border-gray-200 dark:border-gray-700 opacity-60'}`}>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{s.name}</span>
                  {s.enabled ? <span className="text-xs text-green-500 flex items-center gap-0.5"><Check size={10} /> 已启用</span> : <span className="text-xs text-gray-400 flex items-center gap-0.5"><X size={10} /> 已禁用</span>}
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{s.prompt.length > 100 ? s.prompt.slice(0, 100) + '...' : s.prompt}</p>
              </div>
              <div className="flex gap-1 flex-shrink-0">
                <button onClick={() => handleToggle(s)} className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-400" title={s.enabled ? '禁用' : '启用'}>{s.enabled ? <X size={14} /> : <Check size={14} />}</button>
                <button onClick={() => startEdit(s)} className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-400" title="编辑"><Edit2 size={14} /></button>
                <button onClick={() => handleDelete(s.name)} className="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/20 text-red-400" title="删除"><Trash2 size={14} /></button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
