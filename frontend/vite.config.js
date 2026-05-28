import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
//
// server.proxy: фронт во время dev обращается к /api/* и /ws/* как к
// относительным путям. Vite проксирует их на backend.
//
// Target по умолчанию — http://localhost:8080 (когда `npm run dev`
// запускается на хосте). Когда фронт работает в docker compose рядом
// с backend, переменная VITE_BACKEND_URL пробрасывается из compose
// со значением http://backend:8080, потому что внутри docker-сети
// сервисы видят друг друга по имени, а не по localhost.
//
// В production frontend и backend живут за одним nginx, который
// проксирует /api/* — код запросов на фронте одинаковый везде.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backendUrl = env.VITE_BACKEND_URL || 'http://localhost:8080'
  const wsUrl = backendUrl.replace(/^http/, 'ws')

  return {
    plugins: [vue()],
    server: {
      host: '0.0.0.0',
      port: 5173,
      // Для Windows-bind-mount file watcher не получает inotify-события —
      // используем polling. На macOS/Linux polling даёт лишнюю нагрузку,
      // поэтому включаем по env-флагу CHOKIDAR_USEPOLLING.
      watch: {
        usePolling:
          env.CHOKIDAR_USEPOLLING === 'true' ||
          env.CHOKIDAR_USEPOLLING === '1',
      },
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true,
        },
        '/ws': {
          target: wsUrl,
          ws: true,
          changeOrigin: true,
        },
      },
    },
  }
})
