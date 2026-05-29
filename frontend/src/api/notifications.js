import http from './http'

export const notificationsApi = {
  getAll: (params) =>
    http.get('/api/notifications', { params }),

  getUnreadCount: () =>
    http.get('/api/notifications/unread-count'),

  markRead: (id) =>
    http.post(`/api/notifications/${id}/read`),
}
