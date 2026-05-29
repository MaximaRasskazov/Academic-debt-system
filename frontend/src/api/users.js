import http from './http'

export const usersApi = {
  getAll: (params) =>
    http.get('/api/users', { params }),

  getById: (id) =>
    http.get(`/api/users/${id}`),
}
