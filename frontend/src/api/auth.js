import http from './http'

export const authApi = {
  login: (email, password) =>
    http.post('/api/auth/login', { email, password }),

  register: (data) =>
    http.post('/api/auth/register', data),

  logout: () =>
    http.post('/api/auth/logout'),

  me: () =>
    http.get('/api/auth/me'),

  refresh: () =>
    http.post('/api/auth/refresh'),

  changePassword: (currentPassword, newPassword) =>
    http.post('/api/me/password', { current_password: currentPassword, new_password: newPassword }),
}
