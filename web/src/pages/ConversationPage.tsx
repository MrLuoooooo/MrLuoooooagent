import { useNavigate } from 'react-router-dom'
import { useConversations } from '../hooks/useConversations'
import ConversationCard from '../components/ConversationCard'
import { MessageSquare, Plus, Loader2 } from 'lucide-react'

export default function ConversationPage() {
  const convs = useConversations()
  const navigate = useNavigate()

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mx-auto max-w-2xl">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold">会话列表</h1>
          <button
            onClick={() => convs.create('新会话')}
            className="flex items-center gap-2 rounded-xl bg-blue-500 px-4 py-2 text-sm text-white hover:bg-blue-600 transition-colors"
          >
            <Plus size={16} />
            新建会话
          </button>
        </div>

        {convs.loading && (
          <div className="flex justify-center py-12">
            <Loader2 size={24} className="animate-spin text-gray-400" />
          </div>
        )}

        {!convs.loading && convs.conversations.length === 0 && (
          <div className="flex flex-col items-center py-16 text-gray-400">
            <MessageSquare size={48} className="mb-4 opacity-30" />
            <p className="text-lg">暂无会话</p>
            <p className="text-sm mt-1">点击"新建会话"开始对话</p>
          </div>
        )}

        <div className="space-y-2">
          {convs.conversations.map((conv) => (
            <ConversationCard
              key={conv.conversation_id}
              conversation={conv}
              onClick={() => navigate(`/chat?id=${conv.conversation_id}`)}
            />
          ))}
        </div>
      </div>
    </div>
  )
}
