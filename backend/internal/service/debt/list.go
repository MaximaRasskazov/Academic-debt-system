package debt

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

const (
	maxLimit     = 200
	defaultLimit = 50
)

// ListForStudent — все (не удалённые) долги студента.
// Используется в endpoint'е GET /api/debts/my (student, debts.view.own).
func (s *Service) ListForStudent(ctx context.Context, studentID uuid.UUID) ([]queries.Debt, error) {
	rows, err := s.store.ListDebtsForStudent(ctx, pgutil.PgUUID(studentID))
	if err != nil {
		return nil, fmt.Errorf("list for student: %w", err)
	}
	return rows, nil
}

// ListForTeacher — должники по "своим предметам" преподавателя
// (ТЗ: "Преподаватель может посмотреть список должников по своим
// предметам").
//
// "Свой предмет" определяем по debts.issued_by: дисциплина считается
// предметом препода, если он по ней ставил хотя бы один долг. Эмулятор
// деканата не отдаёт явную связь teacher↔discipline (teacher_disciplines
// остаётся пустой), но в каждом долге есть issued_by — это и есть
// "преподаватель, который долг поставил" из ТЗ. Поэтому набор дисциплин
// препода выводим из его долгов, а затем показываем ВСЕХ должников этих
// дисциплин (предмет общий — не только те долги, что препод выставил сам).
//
// Если препод не ставил ни одного долга — у него нет "своих предметов",
// возвращаем пустой список (и без ANY(empty), который дал бы ошибку).
func (s *Service) ListForTeacher(ctx context.Context, teacherID uuid.UUID, limit, offset int32) ([]queries.Debt, error) {
	disciplineIDs, err := s.store.ListDisciplineIDsByIssuer(ctx, pgutil.PgUUID(teacherID))
	if err != nil {
		return nil, fmt.Errorf("list teacher discipline ids: %w", err)
	}
	if len(disciplineIDs) == 0 {
		return nil, nil
	}

	rows, err := s.store.ListDebtsByDisciplines(ctx, queries.ListDebtsByDisciplinesParams{
		DisciplineIds: disciplineIDs,
		Lim:           normalizeLimit(limit),
		Off:           normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list debts by disciplines: %w", err)
	}
	return rows, nil
}

// ListDebtorsByDiscipline — список открытых должников по дисциплине
// с ФИО и группой. Используется фронтом формы заявки на пересдачу:
// преподаватель выбирает кого записать. Без фильтра по
// teacher_disciplines — пока эмулятор эти связи не отдаёт, любой
// preподаватель видит должников по любой дисциплине; декан тоже
// использует это для своего UI создания пересдачи.
func (s *Service) ListDebtorsByDiscipline(ctx context.Context, disciplineID uuid.UUID) ([]queries.ListDebtorsByDisciplineRow, error) {
	rows, err := s.store.ListDebtorsByDiscipline(ctx, pgutil.PgUUID(disciplineID))
	if err != nil {
		return nil, fmt.Errorf("list debtors by discipline: %w", err)
	}
	return rows, nil
}

// ListAll — общий список для деканата.
func (s *Service) ListAll(ctx context.Context, limit, offset int32) ([]queries.Debt, int64, error) {
	rows, err := s.store.ListAllDebts(ctx, queries.ListAllDebtsParams{
		Limit:  normalizeLimit(limit),
		Offset: normalizeOffset(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list all debts: %w", err)
	}
	total, err := s.store.CountDebts(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count debts: %w", err)
	}
	return rows, total, nil
}

// SummaryByDiscipline — для сводной таблицы деканата. Возвращает
// {discipline_id, open_count, graded_count}.
func (s *Service) SummaryByDiscipline(ctx context.Context) ([]queries.SummaryDebtsByDisciplineRow, error) {
	rows, err := s.store.SummaryDebtsByDiscipline(ctx)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	return rows, nil
}

func normalizeLimit(v int32) int32 {
	if v <= 0 {
		return defaultLimit
	}
	if v > maxLimit {
		// Запрос больше максимума упираем в maxLimit, а не схлопываем в
		// defaultLimit — иначе limit=500 молча отдавал бы 50 строк, и
		// постраничная догрузка на фронте останавливалась после первой
		// страницы (декан видел только 50 долгов из всех).
		return maxLimit
	}
	return v
}

func normalizeOffset(v int32) int32 {
	if v < 0 {
		return 0
	}
	return v
}
