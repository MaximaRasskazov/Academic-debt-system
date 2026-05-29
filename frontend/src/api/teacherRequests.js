import api from './client'

// Заявки студентов на роль teacher.
export const teacherRequestsApi = {
  // POST под student. motivation опционально.
  create: (motivation) =>
    api.post('/teacher-requests', { motivation }).then((r) => r.data),

  // Свои заявки (любой авторизованный).
  listMy: () => api.get('/teacher-requests/my').then((r) => r.data),

  // Pending list — только dean (permission teacher_request.review).
  listPending: () => api.get('/teacher-requests').then((r) => r.data),

  approve: (id) =>
    api.post(`/teacher-requests/${id}/approve`, {}).then((r) => r.data),
  reject: (id, reason) =>
    api
      .post(`/teacher-requests/${id}/reject`, { reason })
      .then((r) => r.data),
}

// Заявки преподавателей на изменение пересдачи.
export const changeRequestsApi = {
  submit: (payload) =>
    api.post('/retake-change-requests', payload).then((r) => r.data),
  listPending: () =>
    api.get('/retake-change-requests').then((r) => r.data),
  approve: (id) => api.post(`/retake-change-requests/${id}/approve`),
  reject: (id, reason) =>
    api.post(`/retake-change-requests/${id}/reject`, { reason }),
}
