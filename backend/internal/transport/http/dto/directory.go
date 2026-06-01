// Узкие справочники для UI: списки преподавателей и должников по
// дисциплине без чувствительных полей. Нужны фронту формы заявки на
// пересдачу — преподаватель не имеет users.view и не может тянуть
// общий GET /api/users, но ему нужны id+ФИО других преподавателей
// (для комиссии) и должников выбранной дисциплины.
//
// DTO здесь сознательно минимальны: только то, что нужно UI для
// выбора из выпадающего списка. Никаких email/birthday/created_at —
// преподаватель не должен видеть лишнего по принципу наименьших
// привилегий.
package dto

import (
	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// TeacherBrief — карточка преподавателя для UI-выбора.
type TeacherBrief struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	MiddleName *string   `json:"middle_name,omitempty"`
}

// FromUserToTeacherBrief — из sqlc-структуры User в карточку.
// Email сюда не маппится, оставляем только то что нужно для
// отображения в форме.
func FromUserToTeacherBrief(u queries.User) TeacherBrief {
	return TeacherBrief{
		ID:         pgutil.UUID(u.ID),
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		MiddleName: u.MiddleName,
	}
}

// FromUsersToTeacherBriefs — срез.
func FromUsersToTeacherBriefs(us []queries.User) []TeacherBrief {
	out := make([]TeacherBrief, 0, len(us))
	for _, u := range us {
		out = append(out, FromUserToTeacherBrief(u))
	}
	return out
}

// TeachersListResponse — ответ GET /api/teachers.
type TeachersListResponse struct {
	Items []TeacherBrief `json:"items"`
}

// DebtorBrief — карточка должника + конкретный debt_id, чтобы фронт
// мог сразу передать его в payload заявки на пересдачу (формат
// retake-request требует student_debt_ids[]).
type DebtorBrief struct {
	DebtID     uuid.UUID `json:"debt_id"`
	StudentID  uuid.UUID `json:"student_id"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	MiddleName *string   `json:"middle_name,omitempty"`
	GroupName  *string   `json:"group_name,omitempty"`
}

// FromDebtorRow — sqlc-row → DTO.
func FromDebtorRow(r queries.ListDebtorsByDisciplineRow) DebtorBrief {
	return DebtorBrief{
		DebtID:     pgutil.UUID(r.DebtID),
		StudentID:  pgutil.UUID(r.StudentID),
		FirstName:  r.FirstName,
		LastName:   r.LastName,
		MiddleName: r.MiddleName,
		GroupName:  r.GroupName,
	}
}

// FromDebtorRows — срез.
func FromDebtorRows(rows []queries.ListDebtorsByDisciplineRow) []DebtorBrief {
	out := make([]DebtorBrief, 0, len(rows))
	for _, r := range rows {
		out = append(out, FromDebtorRow(r))
	}
	return out
}

// DebtorsListResponse — ответ GET /api/students/debtors?discipline_id=...
type DebtorsListResponse struct {
	Items []DebtorBrief `json:"items"`
}
