package debt

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

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

// ListForTeacher — долги по всем дисциплинам, которые ведёт teacherID.
// Если у преподавателя нет ни одной дисциплины — возвращаем пустой
// список без обращения к БД. Это и оптимизация, и защита от
// "у преподавателя 0 дисциплин — SELECT WHERE id = ANY(empty) вернёт
// синтаксическую ошибку".
func (s *Service) ListForTeacher(ctx context.Context, teacherID uuid.UUID, limit, offset int32) ([]queries.Debt, error) {
	disciplines, err := s.disciplines.ListDisciplinesForTeacher(ctx, teacherID)
	if err != nil {
		return nil, fmt.Errorf("list teacher disciplines: %w", err)
	}
	if len(disciplines) == 0 {
		return nil, nil
	}

	ids := make([]pgtype.UUID, 0, len(disciplines))
	for _, d := range disciplines {
		ids = append(ids, d.ID)
	}

	rows, err := s.store.ListDebtsByDisciplines(ctx, queries.ListDebtsByDisciplinesParams{
		DisciplineIds: ids,
		Lim:           normalizeLimit(limit),
		Off:           normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list debts by disciplines: %w", err)
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
	if v <= 0 || v > maxLimit {
		return defaultLimit
	}
	return v
}

func normalizeOffset(v int32) int32 {
	if v < 0 {
		return 0
	}
	return v
}
