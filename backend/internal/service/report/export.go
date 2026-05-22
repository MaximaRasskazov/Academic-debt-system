package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExportFormat — какой формат бинарника вернуть.
type ExportFormat string

const (
	FormatXLSX ExportFormat = "xlsx"
	FormatCSV  ExportFormat = "csv"
)

// ContentType возвращает MIME-тип для Content-Type заголовка.
func (f ExportFormat) ContentType() string {
	switch f {
	case FormatXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case FormatCSV:
		return "text/csv; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// FileExt возвращает расширение для filename в Content-Disposition.
func (f ExportFormat) FileExt() string {
	switch f {
	case FormatXLSX:
		return "xlsx"
	case FormatCSV:
		return "csv"
	default:
		return "bin"
	}
}

// reportHeaders — единый заголовок для XLSX/CSV. Меняется здесь —
// меняется во всех экспортах.
var reportHeaders = []string{
	"Дата проведения",
	"Дисциплина (код)",
	"Дисциплина",
	"Тип",
	"Корпус",
	"Аудитория",
	"Продолжительность (мин)",
	"Студент (ФИО)",
	"Группа",
	"Email студента",
	"Оценка",
	"Преподаватели",
}

// ExportRetakes отдаёт байты сериализованного отчёта в указанном
// формате. Если формат не поддержан — fmt.Errorf("unsupported format").
//
// Структура отчёта: одна строка = одна (retake, student) пара. Так
// в Excel/CSV проще делать сводные таблицы и фильтры. Преподаватели
// идут одной ячейкой через запятую — для аналитики обычно достаточно.
func ExportRetakes(rows []RetakeReportRow, format ExportFormat) ([]byte, error) {
	switch format {
	case FormatXLSX:
		return exportXLSX(rows)
	case FormatCSV:
		return exportCSV(rows)
	default:
		return nil, fmt.Errorf("report: unsupported format %q", format)
	}
}

func exportCSV(rows []RetakeReportRow) ([]byte, error) {
	var buf bytes.Buffer
	// UTF-8 BOM в начале, чтобы Excel под Windows корректно открывал
	// кириллицу (без BOM Excel читает как cp1251). На Mac/Linux
	// BOM игнорируется приложениями.
	buf.WriteString("\ufeff")
	w := csv.NewWriter(&buf)

	if err := w.Write(reportHeaders); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}
	for _, r := range rows {
		for _, student := range studentsOrPlaceholder(r) {
			if err := w.Write(formatRow(r, student)); err != nil {
				return nil, fmt.Errorf("write csv row: %w", err)
			}
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}
	return buf.Bytes(), nil
}

func exportXLSX(rows []RetakeReportRow) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheet = "Пересдачи"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, fmt.Errorf("new sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	// Удаляем дефолтный Sheet1 — он пустой и мешает.
	if err := f.DeleteSheet("Sheet1"); err != nil {
		// Не критично — пустой лист рядом не сломает выгрузку.
		_ = err
	}

	// Заголовок.
	for col, header := range reportHeaders {
		cell, err := excelize.CoordinatesToCellName(col+1, 1)
		if err != nil {
			return nil, fmt.Errorf("coord (%d,1): %w", col+1, err)
		}
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, fmt.Errorf("set header cell: %w", err)
		}
	}

	// Данные.
	rowNum := 2
	for _, r := range rows {
		for _, student := range studentsOrPlaceholder(r) {
			values := formatRow(r, student)
			for col, val := range values {
				cell, err := excelize.CoordinatesToCellName(col+1, rowNum)
				if err != nil {
					return nil, fmt.Errorf("coord (%d,%d): %w", col+1, rowNum, err)
				}
				if err := f.SetCellValue(sheet, cell, val); err != nil {
					return nil, fmt.Errorf("set cell: %w", err)
				}
			}
			rowNum++
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

// studentsOrPlaceholder возвращает либо реальный список студентов
// (для каждого — отдельная строка отчёта), либо placeholder из одного
// пустого участника. Без этого пересдачи без студентов (теоретически
// возможны как ошибка деканата) исчезли бы из отчёта.
func studentsOrPlaceholder(r RetakeReportRow) []ParticipantInfo {
	if len(r.Students) == 0 {
		return []ParticipantInfo{{}}
	}
	return r.Students
}

func formatRow(r RetakeReportRow, student ParticipantInfo) []string {
	group := ""
	if student.GroupName != nil {
		group = *student.GroupName
	}
	grade := ""
	if student.Grade != nil {
		grade = strconv.FormatInt(int64(*student.Grade), 10)
	}

	// Преподаватели через запятую.
	teacherNames := make([]string, 0, len(r.Teachers))
	for _, t := range r.Teachers {
		teacherNames = append(teacherNames, t.FullName)
	}

	return []string{
		formatTime(r.CompletedAt),
		r.DisciplineCode,
		r.DisciplineName,
		r.Kind,
		r.Building,
		r.Room,
		strconv.FormatInt(int64(r.DurationMinutes), 10),
		student.FullName,
		group,
		student.Email,
		grade,
		strings.Join(teacherNames, ", "),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}
