import { Outlet, useSearchParams } from 'react-router-dom'
import Sidebar from './Sidebar'
import { Menu } from 'lucide-react'
import { useState, useCallback, useEffect } from 'react'
import { useConversations } from '../hooks/useConversations'

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [searchParams, setSearchParams] = useSearchParams()
  const convId = searchParams.get('id')
  const convs = useConversations()

  // 页面加载时同步 URL 中的会话 ID 到状态
  useEffect(() => {
    if (convId && convId !== convs.currentId) {
      convs.setCurrentId(convId)
    }
  }, [convId]) // eslint-disable-line react-hooks/exhaustive-deps

  // 刷新页面时，自动选中最近一个会话
  useEffect(() => {
    if (!convs.loading && convs.conversations.length > 0 && !convId) {
      const newest = convs.conversations[0].conversation_id
      setSearchParams({ id: newest })
      convs.setCurrentId(newest)
    }
  }, [convs.conversations, convs.loading, convId]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleSelect = useCallback(
    (id: string) => {
      setSearchParams({ id })
      convs.setCurrentId(id)
    },
    [setSearchParams, convs],
  )

  const handleNew = useCallback(async () => {
    const id = await convs.create()
    if (id) {
      setSearchParams({ id })
    }
  }, [convs, setSearchParams])

  return (
    <div className="flex h-screen overflow-hidden">
      {/* 移动端遮罩 */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-20 bg-black/30 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* 侧边栏 */}
      <aside
        className={`fixed inset-y-0 left-0 z-30 w-72 -translate-x-full transition-transform md:relative md:translate-x-0 ${
          sidebarOpen ? 'translate-x-0' : ''
        }`}
      >
        <Sidebar
          conversations={convs.conversations}
          currentId={convId}
          onSelect={handleSelect}
          onNew={handleNew}
          onDelete={convs.delete}
          loading={convs.loading}
          onClose={() => setSidebarOpen(false)}
        />
      </aside>

      {/* 主区域 */}
      <main className="flex flex-1 flex-col min-h-0 min-w-0">
        {/* 顶栏（移动端） */}
        <header className="flex items-center justify-between border-b border-gray-200 px-4 py-2 dark:border-gray-700 md:hidden">
          <button
            onClick={() => setSidebarOpen(true)}
            className="rounded p-1 hover:bg-gray-100 dark:hover:bg-gray-800"
            aria-label="打开菜单"
          >
            <Menu size={20} />
          </button>
          <span className="font-semibold">GoAgent Pro</span>
          <div className="w-8" />
        </header>

        <Outlet context={{ convId, setSearchParams, ...convs }} />
      </main>
    </div>
  )
}
