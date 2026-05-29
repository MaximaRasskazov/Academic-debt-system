import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import router from './router'
import { bindAuthStore } from './api/client'
import { useAuthStore } from './stores/auth'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// API-клиент должен видеть auth-store, чтобы:
//   1) подставлять Bearer-токен в каждый запрос
//   2) дергать setAccessToken/reset из 401-interceptor'а
// Делаем bind после createPinia, иначе useAuthStore() не сможет
// получить активный pinia-инстанс.
const authStore = useAuthStore()
bindAuthStore(authStore)

// При запуске пробуем восстановить сессию — если access лежит в
// localStorage и ещё валиден, /auth/me вернёт профиль; если протух —
// interceptor попробует /refresh по cookie. Только после этого монтируем
// приложение, чтобы router.beforeEach видел корректное состояние.
async function bootstrap() {
  if (authStore.token) {
    try {
      await authStore.fetchMe()
    } catch {
      // reset() уже вызван interceptor'ом, продолжаем монтировать
    }
  } else {
    authStore.initialized = true
  }
  app.mount('#app')
}

bootstrap()
