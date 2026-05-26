package dto

import (
	"time"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/user"
)

// UserListItem — пользователь в списочной выдаче. Включает плоский
// список ролей (только slug+name), потому что фронту обычно достаточно
// для отображения метки "Преподаватель / Студент". Если понадобятся
// permissions — сделаем отдельный endpoint /api/users/:id с расширенным
// профилем.
type UserListItem struct {
	ID         string         `json:"id"`
	Email      string         `json:"email"`
	FirstName  string         `json:"first_name"`
	LastName   string         `json:"last_name"`
	MiddleName *string        `json:"middle_name,omitempty"`
	GroupName  *string        `json:"group_name,omitempty"`
	Birthday   *time.Time     `json:"birthday,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	Roles      []RoleResponse `json:"roles"`
}

// UsersListResponse — пагинированный ответ для GET /api/users.
type UsersListResponse struct {
	Items  []UserListItem `json:"items"`
	Total  int64          `json:"total"`
	Limit  int32          `json:"limit"`
	Offset int32          `json:"offset"`
}

// FromUserListResult конвертирует сервисный тип в DTO.
func FromUserListResult(r *user.ListResult) UsersListResponse {
	items := make([]UserListItem, 0, len(r.Items))
	for _, u := range r.Items {
		roles := make([]RoleResponse, 0, len(u.Roles))
		for _, role := range u.Roles {
			roles = append(roles, FromRole(role))
		}
		base := FromUser(u.User)
		items = append(items, UserListItem{
			ID:         base.ID.String(),
			Email:      base.Email,
			FirstName:  base.FirstName,
			LastName:   base.LastName,
			MiddleName: base.MiddleName,
			GroupName:  base.GroupName,
			Birthday:   base.Birthday,
			CreatedAt:  base.CreatedAt,
			Roles:      roles,
		})
	}
	return UsersListResponse{
		Items:  items,
		Total:  r.Total,
		Limit:  r.Limit,
		Offset: r.Offset,
	}
}

// UpdateProfileRequest — body PATCH /api/me. Все поля опциональны.
// nil = "не трогать", "" = очистить (для middle_name, group_name).
//
// Email сюда сознательно не добавлен — смена email требует подтверждения
// через старую почту, что вне MVP. См. profile.go в auth-сервисе.
type UpdateProfileRequest struct {
	FirstName  *string    `json:"first_name,omitempty"`
	LastName   *string    `json:"last_name,omitempty"`
	MiddleName *string    `json:"middle_name,omitempty"`
	GroupName  *string    `json:"group_name,omitempty"`
	Birthday   *time.Time `json:"birthday,omitempty"`
}

// ChangePasswordRequest — body POST /api/me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}
