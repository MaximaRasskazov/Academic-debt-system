import api from './client'

export const retakesApi = {
  // Пересдачи, где текущий пользователь — участник. Доступно
  // студенту и преподавателю, разный набор полей.
  listMy: () => api.get('/retakes/my').then((r) => r.data),

  // Для dean: все пересдачи. Поддерживает ?status=scheduled|in_progress|...
  // Возвращает { items, limit, offset }.
  listAll: (params = {}) => api.get('/retakes', { params }).then((r) => r.data),

  get: (id) => api.get(`/retakes/${id}`).then((r) => r.data),

  participants: (id) =>
    api.get(`/retakes/${id}/participants`).then((r) => r.data),

  // Создание пересдачи (dean). kind: 'regular' | 'commission'.
  create: (payload) => api.post('/retakes', payload).then((r) => r.data),

  // PATCH-семантика: только переданные поля обновятся.
  update: (id, payload) =>
    api.patch(`/retakes/${id}`, payload).then((r) => r.data),

  start: (id) => api.post(`/retakes/${id}/start`),
  complete: (id) => api.post(`/retakes/${id}/complete`),
  cancel: (id) => api.post(`/retakes/${id}/cancel`),

  addStudent: (id, studentId, debtId) =>
    api.post(`/retakes/${id}/students`, {
      student_id: studentId,
      debt_id: debtId,
    }),
  removeStudent: (id, userId) =>
    api.delete(`/retakes/${id}/students/${userId}`),

  addTeacher: (id, teacherId) =>
    api.post(`/retakes/${id}/teachers`, { teacher_id: teacherId }),
  removeTeacher: (id, userId) =>
    api.delete(`/retakes/${id}/teachers/${userId}`),

  // Атомарно выставляет оценку студенту-участнику и закрывает связанный долг.
  gradeStudent: (id, userId, grade) =>
    api.patch(`/retakes/${id}/students/${userId}/grade`, { grade }),
}
