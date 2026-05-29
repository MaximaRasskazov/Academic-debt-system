import api from './client'

export const authApi = {
  // Возвращает { access_token, user, roles, permissions } — те же поля,
  // что и /auth/me. Backend кладёт refresh_token в HttpOnly cookie.
  login: (email, password) =>
    api.post('/auth/login', { email, password }).then((r) => r.data),

  register: (payload) =>
    api.post('/auth/register', payload).then((r) => r.data),

  // GET /api/auth/me — { user, roles, permissions }. Используется после
  // login и при перезагрузке страницы, если access ещё валиден.
  me: () => api.get('/auth/me').then((r) => r.data),

  // POST /api/auth/refresh — обновить access-токен (cookie шлётся сам).
  // Обычно дёргается из response interceptor'а, но можно и вручную.
  refresh: () => api.post('/auth/refresh').then((r) => r.data),

  // POST /api/auth/logout — ревокация всех access-токенов пользователя.
  logout: () => api.post('/auth/logout'),
}
