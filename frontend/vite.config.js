import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
//
// server.proxy: фронт во время dev обращается к /api/* и /ws/* как к
// относительным путям. Vite проксирует их на локальный backend на :8080.
// В production frontend и backend живут за одним nginx, который тоже
// проксирует /api/*, поэтому код запросов одинаковый везде.
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
        changeOrigin: true,
      },
    },
  },
})
