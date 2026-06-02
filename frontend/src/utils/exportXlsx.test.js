import { describe, it, expect } from 'vitest'
import * as XLSX from 'xlsx'
import { appendAoaSheet } from './exportXlsx'

describe('appendAoaSheet', () => {
  it('добавляет лист с данными, ширинами и объединениями', () => {
    const wb = XLSX.utils.book_new()
    const rows = [
      ['ЗАГОЛОВОК'],
      ['№', 'ФИО', 'Оценка'],
      [1, 'Иванов И.И.', 5],
    ]
    const ws = appendAoaSheet(XLSX, wb, rows, {
      cols: [{ wch: 6 }, { wch: 30 }, { wch: 10 }],
      merges: [{ s: { r: 0, c: 0 }, e: { r: 0, c: 2 } }],
      name: 'Тест',
    })
    expect(wb.SheetNames).toContain('Тест')
    expect(ws['!cols']).toHaveLength(3)
    expect(ws['!merges']).toHaveLength(1)
    expect(ws['A1'].v).toBe('ЗАГОЛОВОК')
    expect(ws['C3'].v).toBe(5)
  })

  it('обрезает имя листа до 31 символа (лимит Excel)', () => {
    const wb = XLSX.utils.book_new()
    appendAoaSheet(XLSX, wb, [['x']], { name: 'a'.repeat(50) })
    expect(wb.SheetNames[0]).toHaveLength(31)
  })

  it('без opts использует дефолтное имя', () => {
    const wb = XLSX.utils.book_new()
    appendAoaSheet(XLSX, wb, [['x']])
    expect(wb.SheetNames[0]).toBe('Лист')
  })
})
