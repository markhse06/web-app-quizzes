import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '')
  const backendURL = env.VITE_BACKEND_URL ?? 'http://localhost:8080'

  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': { target: backendURL, changeOrigin: true },
        '/uploads': { target: backendURL, changeOrigin: true },
        '/ws': { target: backendURL.replace(/^http/, 'ws'), changeOrigin: true, ws: true },
      },
    },
  }
})
