import http from './http'

// Заявки преподавателей на СОЗДАНИЕ пересдачи.
// Отличается от retakeChangeRequests — те про изменение существующей.
//
// Бэк-эндпойнты:
//   POST   /api/retake-requests           — преподаватель подаёт заявку
//   GET    /api/retake-requests/my        — преподаватель смотрит свои
//   GET    /api/retake-requests           — декан смотрит pending
//   POST   /api/retake-requests/:id/approve — декан одобряет (создаёт пересдачу)
//   POST   /api/retake-requests/:id/reject  — декан отклоняет (нужна причина)
export const retakeRequestsApi = {
  submit: (payload) =>
    http.post('/api/retake-requests/', payload),

  listMy: (params) =>
    http.get('/api/retake-requests/my', { params }),

  getAll: (params) =>
    http.get('/api/retake-requests/', { params }),

  approve: (id, decisionReason) =>
    http.post(`/api/retake-requests/${id}/approve`, {
      decision_reason: decisionReason || '',
    }),

  reject: (id, decisionReason) =>
    http.post(`/api/retake-requests/${id}/reject`, {
      decision_reason: decisionReason,
    }),
}
