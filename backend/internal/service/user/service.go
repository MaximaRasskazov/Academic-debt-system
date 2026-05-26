// Package user реализует выборки списков пользователей с фильтрами и
// пагинацией. Не дублирует функционал auth-сервиса (Register/Login/Me) —
// это отдельный admin-style API для деканата:
//
//   - GET /api/users?role=teacher&search=иванов&limit=20 → выбрать
//     преподавателя из выпадающего списка в форме создания пересдачи
//   - GET /api/users?role=student&group_name=ИВТ-21 → ростер группы
//   - админ может смотреть всех без фильтров
//
// Защищается permission'ом users.view (есть у dean и admin из сидов).
package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"

	"github.com/google/uuid"
)

// Service — выборки пользователей.
type Service struct {
	store *repo.Store
}

func New(store *repo.Store) *Service {
	return &Service{store: store}
}

// ListInput — фильтры выборки. Все поля опциональны.
type ListInput struct {
	RoleSlug  string // 'student' / 'teacher' / 'dean' / 'admin' / ''
	Search    string // подстрока для ILIKE по email/first_name/last_name
	GroupName string // точный матч для студентов
	Limit     int32
	Offset    int32
}

// UserWithRoles — пользователь с заранее подтянутыми ролями.
// Сделано отдельным типом, чтобы handler не делал N+1 запросов
// "для каждого user'а получи его роли" — мы это делаем здесь
// одной пачкой.
type UserWithRoles struct {
	User  queries.User
	Roles []queries.Role
}

// ListResult — пагинированный ответ.
type ListResult struct {
	Items  []UserWithRoles
	Total  int64
	Limit  int32
	Offset int32
}

const (
	defaultLimit = int32(50)
	maxLimit     = int32(200)
)

// List возвращает список пользователей с применёнными фильтрами и
// прикреплёнными ролями каждого. Для производительности роли тянем
// одним запросом на пользователя (ListRolesForUser): обычно списки
// небольшие, до 200 строк, и оптимизировать через JOIN агрегацию
// преждевременно — текущая нагрузка курсача это не требует.
func (s *Service) List(ctx context.Context, in ListInput) (*ListResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	params := queries.ListUsersParams{
		LimitN:  limit,
		OffsetN: offset,
	}
	countParams := queries.CountUsersParams{}

	// sqlc ожидает *string для опциональных полей (emit_pointers_for_null_types).
	if v := strings.TrimSpace(in.RoleSlug); v != "" {
		params.RoleSlug = &v
		countParams.RoleSlug = &v
	}
	if v := strings.TrimSpace(in.Search); v != "" {
		params.Search = &v
		countParams.Search = &v
	}
	if v := strings.TrimSpace(in.GroupName); v != "" {
		params.GroupName = &v
		countParams.GroupName = &v
	}

	rows, err := s.store.ListUsers(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	total, err := s.store.CountUsers(ctx, countParams)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	items := make([]UserWithRoles, 0, len(rows))
	for _, u := range rows {
		uid := uuid.UUID(u.ID.Bytes)
		roles, err := s.store.ListRolesForUser(ctx, pgutil.PgUUID(uid))
		if err != nil {
			return nil, fmt.Errorf("list roles for %s: %w", uid, err)
		}
		items = append(items, UserWithRoles{User: u, Roles: roles})
	}

	return &ListResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}
