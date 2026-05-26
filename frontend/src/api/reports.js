import api from './client'

export const reportsApi = {
  debtsSummary: () => api.get('/reports/debts-summary').then((r) => r.data),

  retakes: (params = {}) =>
    api.get('/reports/retakes', { params }).then((r) => r.data),

  // Скачивание XLSX/CSV. Возвращает Blob, дальше можно сохранить через
  // создание ссылки и клик. Если from/to не заданы — backend подставит
  // последний месяц.
  retakesFile: (format, params = {}) =>
    api
      .get('/reports/retakes', {
        params: { ...params, format },
        responseType: 'blob',
      })
      .then((r) => r.data),
}

// Утилита: сохранить Blob на диск. Используется в страницах после
// retakesFile() — фронту нужен только реально полезный wrapper.
export function saveBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
