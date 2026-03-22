import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backend = env.VITE_BACKEND_URL || 'http://localhost:8282'

  return {
    plugins: [react(), tailwindcss()],
    resolve: { alias: { '@': path.resolve(__dirname, './src') } },
    build: {
      outDir: '../pkg/server/assets/build',
      emptyOutDir: true,
    },
    server: {
      proxy: {
        '/api':       { target: backend, changeOrigin: true, cookieDomainRewrite: '' },
        '/debug':     { target: backend, changeOrigin: true, cookieDomainRewrite: '' },
        '/version':   { target: backend, changeOrigin: true, cookieDomainRewrite: '' },
        '/login':     { target: backend, changeOrigin: true, cookieDomainRewrite: '' },
        '/register':  { target: backend, changeOrigin: true, cookieDomainRewrite: '' },
        '/skip-auth': { target: backend, changeOrigin: true, cookieDomainRewrite: '' },
      },
    },
  }
})
