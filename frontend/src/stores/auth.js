// Auth-store: хранит access-токен, профиль и роли/права с backend.
//
// Backend возвращает в /api/auth/me и /login объект вида:
//   {
//     access_token: "...",     // только в /login и /refresh
//     user:        { id, email, first_name, last_name, group_name?, ... },
//     roles:       [{ id, slug: "student", name: "Студент", ... }, ...],
//     permissions: ["debts.view.own", "retakes.view.own", ...]
//   }
//
// Роли в slug используются в lowercase ('student', 'teacher', 'dean',
// 'admin') — это договор с backend, не путать с "STUDENT" из ранней
// версии этого store.
//
// Refresh-токен живёт в HttpOnly cookie, JS его не видит и не должен —
// поэтому в state его нет. Перезагрузка страницы:
//   1. восстанавливаем access из localStorage (если был);
//   2. fetchMe(); если 401 — interceptor дёрнет /refresh по cookie;
//   3. если refresh тоже 401 — reset() + редирект на /login.

import { defineStore } from 'pinia'
import { authApi } from '../api/auth'

const TOKEN_KEY = 'access_token'
const PROFILE_KEY = 'auth_profile'

function loadProfile() {
  try {
    return JSON.parse(localStorage.getItem(PROFILE_KEY) || 'null')
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', {
  state: () => {
    const stored = loadProfile()
    return {
      token: localStorage.getItem(TOKEN_KEY) || null,
      user: stored?.user || null,
      roles: stored?.roles || [], // массив объектов {id, slug, name}
      permissions: stored?.permissions || [], // плоский массив slug-ов
      // initialized — true после первого успешного fetchMe или явного login.
      // Нужен router'у чтобы не рендерить страницы до восстановления сессии.
      initialized: false,
      loading: false,
      error: null,
    }
  },

  getters: {
    isLoggedIn: (s) => !!s.token && !!s.user,
    roleSlugs: (s) => s.roles.map((r) => r.slug),
    // У одного пользователя может быть несколько ролей (например, преподаватель
    // одобрен из студента — у него и student, и teacher). Принципиально показываем
    // "максимальную" по уровню: admin > dean > teacher > student.
    primaryRole(state) {
      const order = ['admin', 'dean', 'teacher', 'student']
      const slugs = state.roles.map((r) => r.slug)
      return order.find((s) => slugs.includes(s)) || null
    },
    isStudent() {
      return this.roleSlugs.includes('student')
    },
    isTeacher() {
      return this.roleSlugs.includes('teacher')
    },
    isDean() {
      return this.roleSlugs.includes('dean')
    },
    isAdmin() {
      return this.roleSlugs.includes('admin')
    },
    fullName: (s) =>
      s.user ? `${s.user.last_name || ''} ${s.user.first_name || ''}`.trim() : '',
  },

  actions: {
    // can(perm) — главный способ проверки прав на UI. Чек идёт по плоскому
    // списку permissions, который пришёл с backend. Использовать как
    // v-if="auth.can('debts.create')" в шаблонах.
    can(slug) {
      return this.permissions.includes(slug)
    },

    setAccessToken(token) {
      this.token = token
      if (token) localStorage.setItem(TOKEN_KEY, token)
      else localStorage.removeItem(TOKEN_KEY)
    },

    applyProfile({ user, roles, permissions }) {
      this.user = user || null
      this.roles = roles || []
      this.permissions = permissions || []
      this.initialized = true
      localStorage.setItem(
        PROFILE_KEY,
        JSON.stringify({ user, roles, permissions }),
      )
    },

    async login(email, password) {
      this.loading = true
      this.error = null
      try {
        const resp = await authApi.login(email, password)
        this.setAccessToken(resp.access_token)
        // /login возвращает только access_token + user, без roles/permissions —
        // их даёт /me. Дёргаем сразу, чтобы routing и UI-проверки прав знали,
        // куда направлять пользователя.
        await this.fetchMe()
        return resp
      } catch (e) {
        // backend возвращает {error, message}. Если 401 — invalid_credentials,
        // 429 — rate_limit_exceeded, и т.д. Отдаём это наружу.
        this.error = e.response?.data?.message || 'Ошибка входа'
        throw e
      } finally {
        this.loading = false
      }
    },

    async fetchMe() {
      this.loading = true
      try {
        const resp = await authApi.me()
        this.applyProfile(resp)
        return resp
      } catch (e) {
        // 401 здесь обработает interceptor → попытается /refresh.
        // Если и он не справится — interceptor вызовет reset() через
        // exposed bindAuthStore.
        this.error = e.response?.data?.message || 'Сессия истекла'
        throw e
      } finally {
        this.loading = false
        this.initialized = true
      }
    },

    async logout() {
      try {
        if (this.token) await authApi.logout()
      } catch {
        // даже если backend ответил ошибкой (например 401) — всё равно
        // чистим локально, чтобы UI не залип в состоянии "вроде залогинен"
      } finally {
        this.reset()
      }
    },

    // reset — синхронный полный сброс. Вызывается interceptor'ом
    // при невозможности refresh-нуть, и из logout().
    reset() {
      this.token = null
      this.user = null
      this.roles = []
      this.permissions = []
      this.error = null
      this.initialized = true
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(PROFILE_KEY)
    },
  },
})
