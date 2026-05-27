import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8081',
        changeOrigin: true,
        // 不缓冲 SSE 流式响应
        configure: (proxy) => {
          proxy.on('proxyRes', (proxyRes) => {
            // 让 Node.js http-proxy 不缓冲 chunked 响应
            proxyRes.headers['cache-control'] = 'no-cache'
          })
        },
      },
    },
  },
})
