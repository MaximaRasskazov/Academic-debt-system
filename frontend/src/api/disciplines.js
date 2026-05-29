import api from './client'

export const disciplinesApi = {
  // Справочник — доступно любому авторизованному.
  list: (params = {}) =>
    api.get('/disciplines', { params }).then((r) => r.data),
  get: (id) => api.get(`/disciplines/${id}`).then((r) => r.data),
  listTeachers: (id) =>
    api.get(`/disciplines/${id}/teachers`).then((r) => r.data),
  listStudents: (id) =>
    api.get(`/disciplines/${id}/students`).then((r) => r.data),

  // Управление — admin/dean.
  create: (payload) => api.post('/disciplines', payload).then((r) => r.data),
  update: (id, payload) =>
    api.patch(`/disciplines/${id}`, payload).then((r) => r.data),
  delete: (id) => api.delete(`/disciplines/${id}`),
  restore: (id) => api.post(`/disciplines/${id}/restore`),

  attachTeacher: (id, teacherId) =>
    api.post(`/disciplines/${id}/teachers`, { teacher_id: teacherId }),
  detachTeacher: (id, userId) =>
    api.delete(`/disciplines/${id}/teachers/${userId}`),

  attachStudent: (id, studentId, academicYear, semester) =>
    api.post(`/disciplines/${id}/students`, {
      student_id: studentId,
      academic_year: academicYear, // строка "2025-2026"
      semester,
    }),
  detachStudent: (id, userId) =>
    api.delete(`/disciplines/${id}/students/${userId}`),

  // Дисциплины пользователя.
  myAsStudent: () => api.get('/me/disciplines/student').then((r) => r.data),
  myAsTeacher: () => api.get('/me/disciplines/teacher').then((r) => r.data),
}
