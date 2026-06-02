// Общие примитивы для генерации .docx через библиотеку `docx`.
//
// Зачем: StatementsPage (ведомость) и DeanPage (сведения о долгах) строили
// одинаковые хелперы (PT/CM, бордеры, фабрики абзаца/ячейки, шапку вуза,
// скачивание блоба) каждый у себя. Здесь — единый источник, без изменения
// формата выгрузки.
//
// `docx` импортируется лениво во вьюхах (`await import('docx')`) ради
// code-splitting, поэтому библиотеки передаются параметром, а не
// импортируются здесь напрямую.

// Перевод единиц для docx: PT — half-points (size), CM — twips.
export const PT = (n) => n * 2
export const CM = (n) => Math.round(n * 567)

const BORDER = { style: 'single', size: 4, color: '000000' }
export const ALL_BORDERS = { top: BORDER, bottom: BORDER, left: BORDER, right: BORDER }

// Фабрика абзаца. opts: align, bold, size(pt), underline, pageBreak,
// before/after (в см). Шрифт — Times New Roman (ГОСТ-стиль документов).
export function makeParagraph(libs, text, opts = {}) {
  const { Paragraph, TextRun, AlignmentType } = libs
  return new Paragraph({
    alignment: opts.align ?? AlignmentType.LEFT,
    pageBreakBefore: !!opts.pageBreak,
    spacing: { before: CM(opts.before ?? 0), after: CM(opts.after ?? 0.18) },
    children: [new TextRun({
      text: String(text),
      bold: !!opts.bold,
      size: PT(opts.size ?? 12),
      font: 'Times New Roman',
      underline: opts.underline ? {} : undefined,
    })],
  })
}

// Фабрика ячейки таблицы. opts: span(columnSpan), w(% ширины), shade,
// align, bold, size(pt).
export function makeCell(libs, text, opts = {}) {
  const { Paragraph, TableCell, TextRun, AlignmentType, WidthType } = libs
  return new TableCell({
    columnSpan: opts.span,
    width: opts.w ? { size: opts.w, type: WidthType.PERCENTAGE } : undefined,
    shading: opts.shade ? { fill: 'EEEEEE' } : undefined,
    borders: ALL_BORDERS,
    children: [new Paragraph({
      alignment: opts.align ?? AlignmentType.LEFT,
      children: [new TextRun({
        text: String(text),
        bold: !!opts.bold,
        size: PT(opts.size ?? 11),
        font: 'Times New Roman',
      })],
    })],
  })
}

// Шапка официального документа вуза (3 центрированных абзаца).
// `title` — название документа (например «ВЕДОМОСТЬ ПЕРЕСДАЧИ»).
export function universityHeader(libs, title, opts = {}) {
  const { AlignmentType } = libs
  return [
    makeParagraph(libs, 'ФЕДЕРАЛЬНОЕ ГОСУДАРСТВЕННОЕ БЮДЖЕТНОЕ ОБРАЗОВАТЕЛЬНОЕ УЧРЕЖДЕНИЕ ВЫСШЕГО ОБРАЗОВАНИЯ', { align: AlignmentType.CENTER, size: opts.orgSize ?? 14 }),
    makeParagraph(libs, 'ЮГОРСКИЙ ГОСУДАРСТВЕННЫЙ УНИВЕРСИТЕТ', { align: AlignmentType.CENTER, size: opts.orgSize ?? 14, after: 0.3 }),
    makeParagraph(libs, title, { align: AlignmentType.CENTER, bold: true, size: 16, before: 0.3, after: opts.titleAfter ?? 0.5 }),
  ]
}

// Стандартная обёртка документа со стилем Times New Roman и полями
// «как в ГОСТ» (3 см слева). `children` — готовый контент секции.
export function makeDocument(libs, children) {
  const { Document } = libs
  return new Document({
    styles: { default: { document: { run: { font: 'Times New Roman', size: PT(12) } } } },
    sections: [{
      properties: { page: { margin: { top: CM(2), right: CM(1.5), bottom: CM(2), left: CM(3) } } },
      children,
    }],
  })
}

// Скачивание готового blob как файла (anchor + ObjectURL).
export function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = Object.assign(document.createElement('a'), { href: url, download: filename })
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
