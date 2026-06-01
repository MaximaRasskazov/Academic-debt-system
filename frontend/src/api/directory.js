import http from './http'

// Узкие справочники для UI: преподаватели и должники по дисциплине.
// Бэк-permission'ы — retakes.request / retakes.create для /teachers,
// debts.view.by_discipline / debts.view.all для /students/debtors.
// Учитель НЕ имеет users.view, поэтому обычный /api/users ему 403 —
// эти эндпойнты сделаны именно для него.
//
// Response shape:
//   GET /api/teachers           → { items: [{ id, first_name, last_name, middle_name? }] }
//   GET /api/students/debtors?discipline_id=...
//                               → { items: [{ debt_id, student_id, first_name, last_name, middle_name?, group_name? }] }
export const directoryApi = {
  listTeachers: () =>
    http.get('/api/teachers'),

  listDebtors: (disciplineId) =>
    http.get('/api/students/debtors', { params: { discipline_id: disciplineId } }),
}
