import { useState, useEffect, useCallback } from 'react'
import { Folder, FolderOpen, File, ChevronRight, ChevronDown, HardDrive } from 'lucide-react'
import { apiFetch } from '../api/client'

interface FileNode {
  name: string
  path: string
  is_dir: boolean
  size?: number
  children?: FileNode[]
}

interface FileBrowserProps {
  workspacePath: string
  onSelectWorkspace: (path: string) => void
  collapsed?: boolean
}

export default function FileBrowser({ workspacePath, onSelectWorkspace, collapsed }: FileBrowserProps) {
  const [drives, setDrives] = useState<FileNode[]>([])
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [children, setChildren] = useState<Record<string, FileNode[]>>({})
  const [showDrives, setShowDrives] = useState(false)

  useEffect(() => {
    apiFetch<FileNode[]>('/workspace/drives').then(setDrives).catch(() => {})
  }, [])

  useEffect(() => {
    if (workspacePath) {
      apiFetch<FileNode[]>(`/workspace/dir?path=${encodeURIComponent(workspacePath)}`)
        .then((nodes) => setChildren({ [workspacePath]: nodes }))
        .catch(() => {})
    }
  }, [workspacePath])

  const toggle = useCallback(async (path: string) => {
    if (expanded[path]) {
      setExpanded((prev) => ({ ...prev, [path]: false }))
      return
    }
    if (!children[path]) {
      try {
        const nodes = await apiFetch<FileNode[]>(`/workspace/dir?path=${encodeURIComponent(path)}`)
        setChildren((prev) => ({ ...prev, [path]: nodes }))
      } catch { return }
    }
    setExpanded((prev) => ({ ...prev, [path]: true }))
  }, [expanded, children])

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes}B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)}MB`
  }

  const renderNode = (node: FileNode, depth: number) => {
    const isExpanded = expanded[node.path]
    const childNodes = children[node.path] || []
    const pad = depth * 16

    return (
      <div key={node.path}>
        <div
          className="flex items-center gap-1 px-2 py-0.5 text-xs cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 rounded"
          style={{ paddingLeft: 8 + pad }}
          onClick={() => node.is_dir ? toggle(node.path) : undefined}
          onDoubleClick={() => node.is_dir && onSelectWorkspace(node.path)}
        >
          {node.is_dir ? (
            isExpanded ? <ChevronDown size={12} className="text-gray-400 shrink-0" /> : <ChevronRight size={12} className="text-gray-400 shrink-0" />
          ) : (
            <span className="w-3 shrink-0" />
          )}
          {node.is_dir ? (
            isExpanded ? <FolderOpen size={14} className="text-amber-500 shrink-0" /> : <Folder size={14} className="text-amber-500 shrink-0" />
          ) : (
            <File size={14} className="text-gray-400 shrink-0" />
          )}
          <span className="truncate text-gray-700 dark:text-gray-300">{node.name}</span>
          {!node.is_dir && node.size !== undefined && (
            <span className="ml-auto text-[10px] text-gray-400 shrink-0">{formatSize(node.size)}</span>
          )}
        </div>
        {node.is_dir && isExpanded && childNodes.map((child) => renderNode(child, depth + 1))}
      </div>
    )
  }

  if (collapsed) return null

  return (
    <div className="flex flex-col h-full border-r border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200 dark:border-gray-700">
        <span className="text-xs font-medium text-gray-500">文件浏览器</span>
        <button
          onClick={() => setShowDrives(!showDrives)}
          className="p-0.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700"
          title="切换磁盘"
        >
          <HardDrive size={14} className="text-gray-400" />
        </button>
      </div>

      {showDrives && (
        <div className="border-b border-gray-200 dark:border-gray-700 p-2">
          <span className="text-[10px] text-gray-400 px-1">磁盘</span>
          <div className="flex flex-wrap gap-1 mt-1">
            {drives.map((d) => (
              <button
                key={d.path}
                onClick={() => {
                  onSelectWorkspace(d.path)
                  setShowDrives(false)
                }}
                className={`px-2 py-0.5 text-[11px] rounded ${
                  workspacePath === d.path
                    ? 'bg-blue-500 text-white'
                    : 'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-300'
                }`}
              >
                {d.name}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="px-2 py-1">
        <span className="text-[10px] text-gray-400 truncate block" title={workspacePath}>
          {workspacePath}
        </span>
      </div>

      <div className="flex-1 overflow-y-auto py-1">
        {workspacePath && children[workspacePath]?.map((node) => renderNode(node, 0))}
        {(!workspacePath || !children[workspacePath]) && (
          <div className="px-3 py-4 text-center text-xs text-gray-400">
            选择一个工作目录
          </div>
        )}
      </div>
    </div>
  )
}
