import http from './http'

// Заявки студентов на роль teacher.
export const teacherRequestsApi = {
  create: (motivation) =>
    http.post('/api/teacher-requests', { motivation }).then((r) => r.data),

  listMy: () => http.get('/api/teacher-requests/my').then((r) => r.data),

  listPending: () => http.get('/api/teacher-requests').then((r) => r.data),

  approve: (id) =>
    http.post(`/api/teacher-requests/${id}/approve`, {}).then((r) => r.data),
  reject: (id, reason) =>
    http.post(`/api/teacher-requests/${id}/reject`, { reason }).then((r) => r.data),
}

// Заявки преподавателей на изменение пересдачи.
export const changeRequestsApi = {
  submit: (payload) =>
    http.post('/api/retake-change-requests', payload).then((r) => r.data),
  listPending: () =>
    http.get('/api/retake-change-requests').then((r) => r.data),
  approve: (id) => http.post(`/api/retake-change-requests/${id}/approve`),
  reject: (id, reason) =>
    http.post(`/api/retake-change-requests/${id}/reject`, { reason }),
}
