// HTTP-клиент для backend API.
//
// Базовый URL `/api` — Vite-proxy в dev и nginx в prod направят на бэкенд.
// `withCredentials: true` нужен чтобы refresh_token-cookie (HttpOnly,
// SameSite=Lax) автоматически летел вместе с запросом на /api/auth/refresh.
//
// Request interceptor подставляет Authorization: Bearer <access_token> из
// auth-store. Если access протух — response interceptor пытается один раз
// дёрнуть /api/auth/refresh, получить новый access, и повторить исходный
// запрос. Если refresh тоже отдал 401 — auth store сбрасывается и роутер
// перебрасывает на /login.

import axios from 'axios'

const apiClient = axios.create({
  baseURL: '/api',
  withCredentials: true,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

// Ссылка на auth-store ставится из main.js после createPinia, чтобы
// избежать циклической зависимости client.js ↔ stores/auth.js.
let authStoreRef = null
export function bindAuthStore(store) {
  authStoreRef = store
}

apiClient.interceptors.request.use((config) => {
  const token = authStoreRef?.token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Один in-flight refresh — чтобы при одновременных 401 от нескольких
// запросов мы не дёргали /refresh 5 раз. Все ждут общего промиса.
let refreshPromise = null

apiClient.interceptors.response.use(
  (resp) => resp,
  async (error) => {
    const original = error.config
    const status = error.response?.status

    // Если 401 и это не сам /auth/refresh — пытаемся обновить токен один раз.
    if (
      status === 401 &&
      original &&
      !original._retry &&
      !original.url?.includes('/auth/refresh') &&
      !original.url?.includes('/auth/login')
    ) {
      original._retry = true

      if (!refreshPromise) {
        refreshPromise = apiClient
          .post('/auth/refresh')
          .then(async (r) => {
            const newToken = r.data.access_token
            authStoreRef?.setAccessToken(newToken)
            // Переподключаем WS с новым токеном. Ленивый импорт разрывает
            // циклическую зависимость client ↔ notifications store.
            try {
              const { useNotificationsStore } = await import('../stores/notifications')
              useNotificationsStore().connect(newToken)
            } catch { /* pinia ещё не инициализирована при cold start */ }
            return newToken
          })
          .catch((e) => {
            authStoreRef?.reset()
            throw e
          })
          .finally(() => {
            refreshPromise = null
          })
      }

      try {
        const newToken = await refreshPromise
        original.headers.Authorization = `Bearer ${newToken}`
        return apiClient.request(original)
      } catch {
        // refresh не удался — пробрасываем оригинальный 401 наверх,
        // store уже сброшен, router выкинет на /login.
        return Promise.reject(error)
      }
    }

    return Promise.reject(error)
  },
)

export default apiClient
