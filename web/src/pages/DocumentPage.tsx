import { useState, useEffect, useCallback } from 'react'
import { uploadDocument, listDocuments } from '../api/document'
import DocumentCard from '../components/DocumentCard'
import { FileUp, Loader2, Upload, AlertCircle } from 'lucide-react'
import type { DocumentItem } from '../types/document'

export default function DocumentPage() {
  const [docs, setDocs] = useState<DocumentItem[]>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(() => {
    setLoading(true)
    listDocuments()
      .then((data) => setDocs(data.documents))
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const handleUpload = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0]
      if (!file) return

      setUploading(true)
      setError(null)
      try {
        await uploadDocument(file, file.name)
        refresh()
      } catch (err) {
        setError((err as Error).message)
      } finally {
        setUploading(false)
      }
    },
    [refresh],
  )

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mx-auto max-w-2xl">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold">文档管理</h1>

          {/* 上传按钮 */}
          <label
            className={`flex items-center gap-2 rounded-xl bg-blue-500 px-4 py-2 text-sm text-white hover:bg-blue-600 cursor-pointer transition-colors ${
              uploading ? 'opacity-50 pointer-events-none' : ''
            }`}
          >
            {uploading ? (
              <Loader2 size={16} className="animate-spin" />
            ) : (
              <Upload size={16} />
            )}
            {uploading ? '上传中…' : '上传文档'}
            <input
              type="file"
              className="hidden"
              accept=".txt,.md,.pdf"
              onChange={handleUpload}
              disabled={uploading}
            />
          </label>
        </div>

        {error && (
          <div className="flex items-center gap-2 bg-red-50 dark:bg-red-900/20 rounded-xl px-4 py-3 mb-4 text-sm text-red-600 dark:text-red-400">
            <AlertCircle size={16} />
            <span>{error}</span>
          </div>
        )}

        {loading && (
          <div className="flex justify-center py-12">
            <Loader2 size={24} className="animate-spin text-gray-400" />
          </div>
        )}

        {!loading && docs.length === 0 && (
          <div className="flex flex-col items-center py-16 text-gray-400">
            <FileUp size={48} className="mb-4 opacity-30" />
            <p className="text-lg">暂无文档</p>
            <p className="text-sm mt-1">上传 .txt / .md / .pdf 文件</p>
          </div>
        )}

        <div className="space-y-3">
          {docs.map((doc) => (
            <DocumentCard key={doc.document_id} document={doc} />
          ))}
        </div>
      </div>
    </div>
  )
}
