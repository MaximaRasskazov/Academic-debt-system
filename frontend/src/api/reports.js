import http from './http'

export const reportsApi = {
  debtsSummary: () =>
    http.get('/api/reports/debts-summary'),

  retakes: (params) =>
    http.get('/api/reports/retakes', { params }),
}
