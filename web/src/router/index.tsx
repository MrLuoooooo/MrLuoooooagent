import { createBrowserRouter, Navigate } from 'react-router-dom'
import Layout from '../components/Layout'
import ChatPage from '../pages/ChatPage'
import ConversationPage from '../pages/ConversationPage'
import DocumentPage from '../pages/DocumentPage'
import SkillPage from '../pages/SkillPage'
import ApprovalPage from '../pages/ApprovalPage'
import BatchPage from '../pages/BatchPage'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <Navigate to="/chat" replace /> },
      { path: 'chat', element: <ChatPage /> },
      { path: 'conversations', element: <ConversationPage /> },
      { path: 'documents', element: <DocumentPage /> },
      { path: 'skills', element: <SkillPage /> },
      { path: 'approvals', element: <ApprovalPage /> },
      { path: 'batch', element: <BatchPage /> },
    ],
  },
])
