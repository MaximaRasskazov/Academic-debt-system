// Общие примитивы для генерации .xlsx через библиотеку `xlsx`.
//
// Как и с docx: библиотека импортируется лениво во вьюхах
// (`await import('xlsx')`) ради code-splitting, поэтому XLSX передаётся
// параметром. Формат выгрузки не меняется — это вынос дублей.

// Добавляет лист из массива-массивов (AOA) в книгу.
//   XLSX     — модуль 'xlsx'
//   wb       — книга (XLSX.utils.book_new())
//   rows     — данные (массив строк-массивов, уже собранный вьюхой)
//   opts.cols   — ширины колонок: [{ wch }]
//   opts.merges — объединения: [{ s:{r,c}, e:{r,c} }]
//   opts.name   — имя листа (обрежется до 31 символа — лимит Excel)
export function appendAoaSheet(XLSX, wb, rows, opts = {}) {
  const ws = XLSX.utils.aoa_to_sheet(rows)
  if (opts.cols) ws['!cols'] = opts.cols
  if (opts.merges) ws['!merges'] = opts.merges
  XLSX.utils.book_append_sheet(wb, ws, (opts.name || 'Лист').slice(0, 31))
  return ws
}
