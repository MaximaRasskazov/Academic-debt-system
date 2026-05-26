package handler

import (
	"errors"
	"net/http"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/user"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/transport/http/dto"
)

// UsersHandler — обёртка GET /api/users с фильтрами.
type UsersHandler struct {
	svc *user.Service
}

func NewUsersHandler(svc *user.Service) *UsersHandler {
	return &UsersHandler{svc: svc}
}

// List godoc
//
//	@Summary	Список пользователей (admin / dean)
//	@Tags		users
//	@Param		role		query	string	false	"Фильтр по slug роли: student / teacher / dean / admin"
//	@Param		search		query	string	false	"Подстрока по email / first_name / last_name"
//	@Param		group_name	query	string	false	"Точный матч по group_name (для студентов)"
//	@Param		limit		query	int		false	"Лимит (по умолчанию 50, max 200)"
//	@Param		offset		query	int		false	"Смещение"
//	@Success	200	{object}	dto.UsersListResponse
//	@Failure	403	{object}	dto.ErrorResponse
//	@Security	BearerAuth
//	@Router		/api/users [get]
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := user.ListInput{
		RoleSlug:  q.Get("role"),
		Search:    q.Get("search"),
		GroupName: q.Get("group_name"),
		Limit:     parseInt32(q.Get("limit"), 50),
		Offset:    parseInt32(q.Get("offset"), 0),
	}

	result, err := h.svc.List(r.Context(), in)
	if err != nil {
		// Здесь специфичных sentinel-ошибок нет — все ошибки сервиса
		// внутренние (БД). Возвращаем 500.
		_ = errors.Unwrap(err) // подавляем ineffassign в линтере для err — это документально
		writeError(w, http.StatusInternalServerError, "internal", "не удалось получить список пользователей")
		return
	}
	writeJSON(w, http.StatusOK, dto.FromUserListResult(result))
}
