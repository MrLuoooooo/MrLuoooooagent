import { Outlet, useSearchParams } from 'react-router-dom'
import Sidebar from './Sidebar'
import TopNav from './TopNav'
import { PanelLeftClose, PanelLeft } from 'lucide-react'
import { useState, useCallback, useEffect } from 'react'
import { useConversations } from '../hooks/useConversations'

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [searchParams, setSearchParams] = useSearchParams()
  const convId = searchParams.get('id')
  const convs = useConversations()

  useEffect(() => {
    if (convId && convId !== convs.currentId) {
      convs.setCurrentId(convId)
    }
  }, [convId]) // eslint-disable-line react-hooks/exhaustive-deps

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
    <div className="flex h-screen overflow-hidden bg-gray-50 dark:bg-gray-950">
      <aside
        className={`hidden md:flex flex-col border-r border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 transition-all duration-200 ${
          sidebarOpen ? 'w-72' : 'w-0 min-w-0 overflow-hidden border-r-0'
        }`}
      >
        <div className="flex-shrink-0 flex items-center justify-between px-3 py-2.5 border-b border-gray-100 dark:border-gray-800">
          {sidebarOpen && (
            <span className="text-xs font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wider">
              会话历史
            </span>
          )}
          <button
            onClick={() => setSidebarOpen(false)}
            className="rounded-lg p-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors ml-auto"
            title="收起侧边栏"
          >
            <PanelLeftClose size={16} />
          </button>
        </div>
        <Sidebar
          conversations={convs.conversations}
          currentId={convId}
          onSelect={handleSelect}
          onNew={handleNew}
          onDelete={convs.delete}
          loading={convs.loading}
          onClose={() => {}}
        />
      </aside>

      <main className="flex flex-1 flex-col min-h-0 min-w-0 bg-white dark:bg-gray-950">
        <TopNav />

        {!sidebarOpen && (
          <button
            onClick={() => setSidebarOpen(true)}
            className="hidden md:flex fixed left-3 top-14 z-10 rounded-lg bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 shadow-sm p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:shadow-md transition-all"
            title="展开侧边栏"
          >
            <PanelLeft size={16} />
          </button>
        )}

        <div className="hidden md:block h-[1px] bg-gradient-to-r from-blue-100 via-purple-100 to-transparent dark:from-blue-900/20 dark:via-purple-900/20" />

        <Outlet context={{ convId, setSearchParams, ...convs }} />
      </main>
    </div>
  )
}
