import http from './http'

export const debtsApi = {
  // Декан — все долги
  getAll: (params) =>
    http.get('/api/debts', { params }),

  // Студент — свои долги
  getMy: () =>
    http.get('/api/debts/my'),

  // Преподаватель — долги по своим дисциплинам
  getByDiscipline: (params) =>
    http.get('/api/debts/by-discipline', { params }),

  listByDiscipline: (params) =>
    http.get('/api/debts/by-discipline', { params }),

  // Сводка по дисциплинам
  getSummary: () =>
    http.get('/api/debts/summary'),

  getById: (id) =>
    http.get(`/api/debts/${id}`),

  // Преподаватель — поставить долг
  create: (data) =>
    http.post('/api/debts', data),

  grade: (id, grade) =>
    http.patch(`/api/debts/${id}/grade`, { grade }),

  cancel: (id) =>
    http.patch(`/api/debts/${id}/cancel`),
}
