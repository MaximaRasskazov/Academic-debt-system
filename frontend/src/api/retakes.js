import http from './http'

export const retakesApi = {
  // Декан — все пересдачи
  getAll: (params) =>
    http.get('/api/retakes', { params }),

  // Студент/преподаватель — свои пересдачи
  getMy: () =>
    http.get('/api/retakes/my'),

  getById: (id) =>
    http.get(`/api/retakes/${id}`),

  create: (data) =>
    http.post('/api/retakes', data),

  update: (id, data) =>
    http.patch(`/api/retakes/${id}`, data),

  start: (id) =>
    http.post(`/api/retakes/${id}/start`),

  complete: (id) =>
    http.post(`/api/retakes/${id}/complete`),

  cancel: (id) =>
    http.post(`/api/retakes/${id}/cancel`),

  getParticipants: (id) =>
    http.get(`/api/retakes/${id}/participants`),

  addStudent: (id, data) =>
    http.post(`/api/retakes/${id}/students`, data),

  removeStudent: (id, userId) =>
    http.delete(`/api/retakes/${id}/students/${userId}`),

  gradeStudent: (id, userId, grade) =>
    http.patch(`/api/retakes/${id}/students/${userId}/grade`, { grade }),

  addTeacher: (id, teacherId) =>
    http.post(`/api/retakes/${id}/teachers`, { teacher_id: teacherId }),

  removeTeacher: (id, userId) =>
    http.delete(`/api/retakes/${id}/teachers/${userId}`),
}
