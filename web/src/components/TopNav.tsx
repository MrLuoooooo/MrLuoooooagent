import { NavLink, useLocation } from 'react-router-dom'
import { Bot, MessageSquare, FolderOpen, Puzzle, Shield, Layers, TrendingUp, ChevronDown, Menu, X, type LucideIcon } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'

const mainLinks = [
  { to: '/chat', label: '对话', icon: Bot },
  { to: '/stock', label: '股票', icon: TrendingUp },
  { to: '/conversations', label: '会话', icon: MessageSquare },
  { to: '/documents', label: '文档', icon: FolderOpen },
]

const agentLinks = [
  { to: '/skills', label: '技能管理', icon: Puzzle, desc: '自定义 Agent 提示词' },
  { to: '/approvals', label: '审批中心', icon: Shield, desc: '工具调用审批' },
  { to: '/batch', label: '批量任务', icon: Layers, desc: '批量执行 Prompt' },
]

function isAgentRoute(pathname: string) {
  return agentLinks.some((l) => l.to === pathname)
}

function NavLinkItem({ to, icon: Icon, label, onClick }: { to: string; icon: LucideIcon; label: string; onClick?: () => void }) {
  return (
    <NavLink
      to={to}
      onClick={onClick}
      className={({ isActive }) =>
        `flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition-all duration-150 ${
          isActive
            ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300 shadow-sm'
            : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-900 dark:hover:text-gray-200'
        }`
      }
    >
      <Icon size={16} />
      <span>{label}</span>
    </NavLink>
  )
}

export default function TopNav() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const location = useLocation()
  const dropdownRef = useRef<HTMLDivElement>(null)

  const inAgent = isAgentRoute(location.pathname)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  useEffect(() => {
    setMobileOpen(false)
    setDropdownOpen(false)
  }, [location.pathname])

  return (
    <>
      <header className="hidden md:flex items-center justify-between border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 px-4 h-12 flex-shrink-0">
        <div className="flex items-center gap-1">
          <NavLink
            to="/chat"
            className="flex items-center gap-1.5 mr-3 font-bold text-base text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
          >
            <div className="w-6 h-6 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
              <span className="text-white text-[10px] font-bold">G</span>
            </div>
            GoAgent
          </NavLink>

          <nav className="flex items-center gap-0.5">
            {mainLinks.map((link) => (
              <NavLinkItem key={link.to} {...link} />
            ))}

            <div ref={dropdownRef} className="relative">
              <button
                onClick={() => setDropdownOpen(!dropdownOpen)}
                className={`flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium transition-all duration-150 ${
                  inAgent
                    ? 'bg-purple-100 dark:bg-purple-900/40 text-purple-700 dark:text-purple-300 shadow-sm'
                    : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-900 dark:hover:text-gray-200'
                }`}
              >
                <Bot size={16} />
                <span>Agent 应用</span>
                <ChevronDown
                  size={14}
                  className={`transition-transform duration-200 ${dropdownOpen ? 'rotate-180' : ''}`}
                />
              </button>

              {dropdownOpen && (
                <div className="absolute top-full left-0 mt-1 w-48 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 shadow-xl shadow-black/5 dark:shadow-black/20 py-1.5 z-50 animate-in fade-in slide-in-from-top-1 duration-200">
                  {agentLinks.map((link) => (
                    <NavLink
                      key={link.to}
                      to={link.to}
                      onClick={() => setDropdownOpen(false)}
                      className={({ isActive }) =>
                        `flex items-center gap-2 px-3 py-2 text-sm transition-colors ${
                          isActive
                            ? 'bg-purple-50 dark:bg-purple-900/20 text-purple-700 dark:text-purple-300 font-medium'
                            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
                        }`
                      }
                    >
                      <link.icon size={16} className="flex-shrink-0 opacity-70" />
                      <div className="flex flex-col">
                        <span>{link.label}</span>
                        <span className="text-[10px] text-gray-400 dark:text-gray-500">{link.desc}</span>
                      </div>
                    </NavLink>
                  ))}
                </div>
              )}
            </div>
          </nav>
        </div>

        <span className="text-xs text-gray-400 dark:text-gray-500">
          v1.0
        </span>
      </header>

      <header className="flex md:hidden items-center justify-between border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-950 px-4 py-2.5 flex-shrink-0">
        <button
          onClick={() => setMobileOpen(true)}
          className="rounded-lg p-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 -ml-1.5"
          aria-label="打开菜单"
        >
          <Menu size={20} className="text-gray-700 dark:text-gray-300" />
        </button>
        <NavLink to="/chat" className="flex items-center gap-1.5 font-bold text-base text-gray-900 dark:text-gray-100">
          <div className="w-6 h-6 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
            <span className="text-white text-[10px] font-bold">G</span>
          </div>
          GoAgent
        </NavLink>
        <div className="w-8" />
      </header>

      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 md:hidden"
          onClick={() => setMobileOpen(false)}
        >
          <div className="absolute inset-0 bg-black/30" />
        </div>
      )}

      <aside
        className={`fixed inset-y-0 left-0 z-50 w-72 bg-white dark:bg-gray-950 border-r border-gray-200 dark:border-gray-800 shadow-xl transform transition-transform duration-300 md:hidden ${
          mobileOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-800">
          <div className="flex items-center gap-1.5 font-bold text-base text-gray-900 dark:text-gray-100">
            <div className="w-6 h-6 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
              <span className="text-white text-[10px] font-bold">G</span>
            </div>
            GoAgent
          </div>
          <button
            onClick={() => setMobileOpen(false)}
            className="rounded-lg p-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 -mr-1.5"
          >
            <X size={18} className="text-gray-500" />
          </button>
        </div>

        <div className="p-3 space-y-1 overflow-y-auto h-full pb-20">
          <p className="text-xs font-medium text-gray-400 dark:text-gray-500 px-2 py-1 uppercase tracking-wider">主菜单</p>
          {mainLinks.map((link) => (
            <NavLinkItem key={link.to} {...link} onClick={() => setMobileOpen(false)} />
          ))}

          <div className="pt-3 pb-1">
            <p className="text-xs font-medium text-gray-400 dark:text-gray-500 px-2 py-1 uppercase tracking-wider">
              Agent 应用
            </p>
          </div>
          {agentLinks.map((link) => (
            <NavLinkItem key={link.to} {...link} onClick={() => setMobileOpen(false)} />
          ))}
        </div>
      </aside>
    </>
  )
}
