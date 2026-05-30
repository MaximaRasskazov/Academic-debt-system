package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/user"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
)

// DirectoryUsers / DirectoryDebts — что нужно handler'у от сервисов.
// Объявлены здесь (не в пакетах user/debt), чтобы в тестах можно было
// подсунуть stub без поднятия БД. Тот же паттерн, что в retake.go и
// retake_request.go.
type DirectoryUsers interface {
	List(ctx context.Context, in user.ListInput) (*user.ListResult, error)
}

type DirectoryDebts interface {
	ListDebtorsByDiscipline(ctx context.Context, disciplineID uuid.UUID) ([]queries.ListDebtorsByDisciplineRow, error)
}

// DirectoryHandler — узкие справочники для UI: список преподавателей
// (для составления комиссии) и список должников по дисциплине (для
// записи студентов на пересдачу).
//
// Сделан отдельным handler-ом, а не методом UsersHandler/DebtHandler,
// чтобы permission-границы в роутере оставались явными: эти эндпойнты
// разрешены под retakes.request, а оригинальные /api/users и
// /api/debts/by-discipline — нет (там users.view и debts.view.by_discipline).
type DirectoryHandler struct {
	users DirectoryUsers
	debts DirectoryDebts
}

func NewDirectoryHandler(users DirectoryUsers, debts DirectoryDebts) *DirectoryHandler {
	return &DirectoryHandler{users: users, debts: debts}
}

// ListTeachers godoc
//
//	@Summary	Список преподавателей для UI (составление комиссии)
//	@Tags		directory
//	@Produce	json
//	@Success	200	{object}	dto.TeachersListResponse
//	@Failure	401	{object}	dto.ErrorResponse
//	@Failure	403	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/teachers [get]
func (h *DirectoryHandler) ListTeachers(w http.ResponseWriter, r *http.Request) {
	// Заюзаем существующий user.Service.List с фильтром role=teacher.
	// Limit=200 — у нас всего 12 преподавателей в эмуляторе, запас x16.
	// Если когда-то будет больше 200 — пагинацию добавим (а на UI всё
	// равно нужен поиск).
	result, err := h.users.List(r.Context(), user.ListInput{
		RoleSlug: "teacher",
		Limit:    200,
		Offset:   0,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось получить список преподавателей")
		return
	}

	teachers := make([]dto.TeacherBrief, 0, len(result.Items))
	for _, ur := range result.Items {
		teachers = append(teachers, dto.FromUserToTeacherBrief(ur.User))
	}
	writeJSON(w, http.StatusOK, dto.TeachersListResponse{Items: teachers})
}

// ListDebtors godoc
//
//	@Summary	Должники по дисциплине (для формы заявки на пересдачу)
//	@Tags		directory
//	@Produce	json
//	@Param		discipline_id	query		string	true	"UUID дисциплины"
//	@Success	200				{object}	dto.DebtorsListResponse
//	@Failure	400				{object}	dto.ErrorResponse
//	@Failure	401				{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/students/debtors [get]
func (h *DirectoryHandler) ListDebtors(w http.ResponseWriter, r *http.Request) {
	rawID := r.URL.Query().Get("discipline_id")
	if rawID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "discipline_id обязателен")
		return
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "discipline_id должен быть UUID")
		return
	}

	rows, err := h.debts.ListDebtorsByDiscipline(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "не удалось получить список должников")
		return
	}
	writeJSON(w, http.StatusOK, dto.DebtorsListResponse{Items: dto.FromDebtorRows(rows)})
}
