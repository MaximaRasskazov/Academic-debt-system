import api from './client'

// Список пользователей — нужен фронту для multiselect преподавателей при
// создании пересдачи, выбора студентов и в админ-панели.
// Бэкенд: GET /api/users (permission users.view — admin/dean).
export const usersApi = {
  // params: { role?: 'teacher'|'student'|..., search?, group_name?, limit?, offset? }
  list: (params = {}) => api.get('/users', { params }).then((r) => r.data),
}
