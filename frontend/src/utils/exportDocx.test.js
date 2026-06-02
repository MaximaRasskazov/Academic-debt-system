import { describe, it, expect } from 'vitest'
import * as docx from 'docx'
import { PT, CM, makeParagraph, makeCell, universityHeader, makeDocument } from './exportDocx'

describe('единицы docx', () => {
  it('PT — half-points', () => expect(PT(12)).toBe(24))
  it('CM — twips', () => {
    expect(CM(1)).toBe(567)
    expect(CM(2)).toBe(1134)
  })
})

describe('фабрики docx (smoke — собирают валидные объекты)', () => {
  it('makeParagraph возвращает Paragraph', () => {
    expect(makeParagraph(docx, 'текст', { bold: true })).toBeInstanceOf(docx.Paragraph)
  })
  it('makeCell возвращает TableCell', () => {
    expect(makeCell(docx, 'ячейка', { w: 30, shade: true })).toBeInstanceOf(docx.TableCell)
  })
  it('universityHeader — три абзаца с заголовком', () => {
    const head = universityHeader(docx, 'ВЕДОМОСТЬ ПЕРЕСДАЧИ')
    expect(head).toHaveLength(3)
    head.forEach(p => expect(p).toBeInstanceOf(docx.Paragraph))
  })
  it('makeDocument возвращает Document', () => {
    const doc = makeDocument(docx, universityHeader(docx, 'X'))
    expect(doc).toBeInstanceOf(docx.Document)
  })
})
