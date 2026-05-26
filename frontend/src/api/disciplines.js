import http from './http'

export const disciplinesApi = {
  getAll: (params) =>
    http.get('/api/disciplines', { params }),

  getById: (id) =>
    http.get(`/api/disciplines/${id}`),

  create: (data) =>
    http.post('/api/disciplines', data),

  update: (id, data) =>
    http.patch(`/api/disciplines/${id}`, data),

  remove: (id) =>
    http.delete(`/api/disciplines/${id}`),

  restore: (id) =>
    http.post(`/api/disciplines/${id}/restore`),

  // Студенты дисциплины
  getStudents: (id) =>
    http.get(`/api/disciplines/${id}/students`),

  attachStudent: (id, data) =>
    http.post(`/api/disciplines/${id}/students`, data),

  detachStudent: (id, userId) =>
    http.delete(`/api/disciplines/${id}/students/${userId}`),

  // Преподаватели дисциплины
  getTeachers: (id) =>
    http.get(`/api/disciplines/${id}/teachers`),

  attachTeacher: (id, teacherId) =>
    http.post(`/api/disciplines/${id}/teachers`, { teacher_id: teacherId }),

  detachTeacher: (id, userId) =>
    http.delete(`/api/disciplines/${id}/teachers/${userId}`),

  // Мои дисциплины
  myAsStudent: () =>
    http.get('/api/me/disciplines/student'),

  myAsTeacher: () =>
    http.get('/api/me/disciplines/teacher'),

  // Дисциплины конкретного пользователя (декан)
  userAsStudent: (userId) =>
    http.get(`/api/users/${userId}/disciplines/student`),

  userAsTeacher: (userId) =>
    http.get(`/api/users/${userId}/disciplines/teacher`),
}
