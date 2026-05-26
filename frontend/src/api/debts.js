import api from './client'

export const debtsApi = {
  // Для роли student. Бэкенд возвращает массив объектов debt.
  listMy: () => api.get('/debts/my').then((r) => r.data),

  // Для роли teacher. Долги по дисциплинам, которые он ведёт.
  listByDiscipline: (params = {}) =>
    api.get('/debts/by-discipline', { params }).then((r) => r.data),

  // Для dean/admin. Возвращает { items, total, limit, offset }.
  listAll: (params = {}) => api.get('/debts', { params }).then((r) => r.data),

  // Сводка open/graded по дисциплинам — для dean.
  summary: () => api.get('/debts/summary').then((r) => r.data),

  get: (id) => api.get(`/debts/${id}`).then((r) => r.data),

  // Создание долга (teacher). Бэкенд проверяет teacher_disciplines +
  // student_disciplines, может вернуть 403/422/409.
  create: (payload) => api.post('/debts', payload).then((r) => r.data),

  // Выставить оценку 2..5. Возвращает обновлённый debt со status=graded.
  grade: (id, grade) =>
    api.patch(`/debts/${id}/grade`, { grade }).then((r) => r.data),

  // Отменить долг (dean). 204 No Content.
  cancel: (id) => api.patch(`/debts/${id}/cancel`),
}
