import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // All /api/* traffic flows through the gateway, which handles:
      //   /api/auth/* → auth-service :8000
      //   /api/todos* → todo-service :8001
      // plus rate limiting, CORS, request logging.
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
