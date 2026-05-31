// Экспорт таблицы в Excel (.xls via SpreadsheetML) и Word (.doc via HTML).
// Не требует внешних библиотек — всё через data URI + Blob.

function downloadBlob(content, filename, mimeType) {
  const blob = new Blob(['﻿' + content], { type: mimeType + ';charset=utf-8;' })
  const url  = URL.createObjectURL(blob)
  const a    = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

function escHtml(s) {
  return String(s ?? '').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')
}

// ── Excel (SpreadsheetML / .xls) ────────────────────────────────────────────
export function exportExcel(title, headers, rows, filename = 'export') {
  const thead = headers.map(h => `<Cell><Data ss:Type="String">${escHtml(h)}</Data></Cell>`).join('')
  const tbody = rows.map(row =>
    '<Row>' + row.map(cell => `<Cell><Data ss:Type="String">${escHtml(cell)}</Data></Cell>`).join('') + '</Row>'
  ).join('')

  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<?mso-application progid="Excel.Sheet"?>
<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet"
          xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">
  <Styles>
    <Style ss:ID="h">
      <Font ss:Bold="1" ss:Size="11"/>
      <Interior ss:Color="#E8EAFF" ss:Pattern="Solid"/>
    </Style>
    <Style ss:ID="t">
      <Font ss:Bold="1" ss:Size="13"/>
    </Style>
  </Styles>
  <Worksheet ss:Name="Данные">
    <Table>
      <Row><Cell ss:MergeAcross="${headers.length - 1}" ss:StyleID="t"><Data ss:Type="String">${escHtml(title)}</Data></Cell></Row>
      <Row>${headers.map(h => `<Cell ss:StyleID="h"><Data ss:Type="String">${escHtml(h)}</Data></Cell>`).join('')}</Row>
      ${tbody}
    </Table>
  </Worksheet>
</Workbook>`

  downloadBlob(xml, filename + '.xls', 'application/vnd.ms-excel')
}

// ── Word (.doc via HTML) ─────────────────────────────────────────────────────
export function exportWord(title, headers, rows, filename = 'export') {
  const thCells = headers.map(h => `<th>${escHtml(h)}</th>`).join('')
  const tbRows  = rows.map(row =>
    '<tr>' + row.map(cell => `<td>${escHtml(cell)}</td>`).join('') + '</tr>'
  ).join('')

  const html = `<html xmlns:o="urn:schemas-microsoft-com:office:office"
      xmlns:w="urn:schemas-microsoft-com:office:word"
      xmlns="http://www.w3.org/TR/REC-html40">
<head><meta charset="utf-8"/><title>${escHtml(title)}</title>
<style>
  body { font-family: Calibri, Arial, sans-serif; font-size: 11pt; margin: 24pt; }
  h2   { font-size: 14pt; color: #3C38B6; margin-bottom: 12pt; }
  table { border-collapse: collapse; width: 100%; }
  th { background: #E8EAFF; font-weight: bold; padding: 6pt 8pt; border: 1pt solid #c5c8d4; text-align: left; }
  td { padding: 5pt 8pt; border: 1pt solid #d7d9e0; vertical-align: top; }
  tr:nth-child(even) td { background: #f7f8ff; }
</style></head>
<body>
<h2>${escHtml(title)}</h2>
<table><thead><tr>${thCells}</tr></thead><tbody>${tbRows}</tbody></table>
</body></html>`

  downloadBlob(html, filename + '.doc', 'application/msword')
}
