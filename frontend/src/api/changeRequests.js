import http from './http'

export const changeRequestsApi = {
  // Декан — список pending-заявок
  getAll: (params) =>
    http.get('/api/retake-change-requests', { params }),

  // Преподаватель — подать заявку на изменение
  submit: (data) =>
    http.post('/api/retake-change-requests', data),

  // Декан — одобрить. selectedSlot — ISO-строка выбранной даты для заявок
  // с несколькими предложенными вариантами (proposed_slots).
  approve: (id, decisionReason, selectedSlot) =>
    http.post(`/api/retake-change-requests/${id}/approve`, {
      decision_reason: decisionReason || '',
      ...(selectedSlot ? { selected_slot: selectedSlot } : {}),
    }),

  // Декан — отклонить (decision_reason обязателен)
  reject: (id, decisionReason) =>
    http.post(`/api/retake-change-requests/${id}/reject`, { decision_reason: decisionReason }),
}
