// Package auth реализует регистрацию, вход, refresh и logout пользователей.
//
// Сервис тонкий: проверяет уникальность email, хеширует пароль через
// bcrypt, делегирует выдачу токенов TokenService и атомарно через UoW
// связывает нового пользователя с ролью student.
package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/token"
)

// Дефолтная роль, выдаваемая при регистрации. После регистрации
// пользователь может подать заявку на роль teacher (отдельный поток).
const defaultRoleSlug = "student"

// MinPasswordLength — минимальная длина пароля при регистрации и смене.
// На фронте дублируется через zod-схему.
const MinPasswordLength = 8

// emailRegex — упрощённая проверка email-формата. Финальную валидацию
// делает БД (UNIQUE-индекс) и провайдер при отправке писем.
var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Sentinel-ошибки, на которые HTTP-слой маппит коды ответа.
var (
	ErrEmailTaken         = errors.New("auth: email уже зарегистрирован")
	ErrInvalidEmail       = errors.New("auth: некорректный email")
	ErrPasswordTooShort   = errors.New("auth: пароль слишком короткий")
	ErrInvalidCredentials = errors.New("auth: неверный email или пароль")
	ErrUserNotFound       = errors.New("auth: пользователь не найден")
)

// Service — обёртка над хранилищем и TokenService.
type Service struct {
	store  *repo.Store
	tokens *token.Service
}

// New собирает Service. defaultRole загружается лениво при первом
// вызове Register — это позволяет создать сервис до накатывания
// миграций (полезно для unit-тестов на конфиг).
func New(store *repo.Store, tokens *token.Service) *Service {
	return &Service{store: store, tokens: tokens}
}

// RegisterInput — параметры регистрации. MiddleName/Birthday/GroupName
// опциональны (студенту нужна группа, но мы не валидируем это здесь —
// форма на фронте сама подскажет).
type RegisterInput struct {
	Email      string
	Password   string
	FirstName  string
	LastName   string
	MiddleName *string
	Birthday   *time.Time
	GroupName  *string
}

// Result — результат успешного Register / Login.
type Result struct {
	User queries.User
	Pair *token.Pair
}

// Profile — данные для GET /api/auth/me.
// Раздаёт минимум, нужный фронту для отрисовки UI и RBAC-проверок.
type Profile struct {
	User        queries.User
	Roles       []queries.Role
	Permissions []string // slug'и активных permission'ов
}

// Register создаёт нового пользователя со студентской ролью и сразу
// выдаёт пару токенов. ip пробрасывается в access_tokens для аудита.
func (s *Service) Register(ctx context.Context, in RegisterInput, ip string) (*Result, error) {
	if err := validateRegister(in); err != nil {
		return nil, err
	}
	emailLower := strings.ToLower(strings.TrimSpace(in.Email))

	// Дубликат по email отсекаем заранее — иначе bcrypt напрасно сожгёт CPU.
	if _, err := s.store.GetUserByEmail(ctx, emailLower); err == nil {
		return nil, ErrEmailTaken
	} else if !repo.IsNotFound(err) {
		return nil, fmt.Errorf("lookup email: %w", err)
	}

	passHash, err := hashPassword(in.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	studentRole, err := s.store.GetRoleBySlug(ctx, defaultRoleSlug)
	if err != nil {
		return nil, fmt.Errorf("lookup default role %q: %w", defaultRoleSlug, err)
	}

	var created queries.User
	err = s.store.RunInTx(ctx, func(q *queries.Queries) error {
		u, err := q.CreateUser(ctx, queries.CreateUserParams{
			Email:        emailLower,
			PasswordHash: passHash,
			FirstName:    strings.TrimSpace(in.FirstName),
			LastName:     strings.TrimSpace(in.LastName),
			MiddleName:   in.MiddleName,
			Birthday:     dateOrNull(in.Birthday),
			GroupName:    in.GroupName,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		_, err = q.AttachRoleToUser(ctx, queries.AttachRoleToUserParams{
			UserID:    u.ID,
			RoleID:    studentRole.ID,
			CreatedBy: u.ID, // сам себя «создал» — для системного актора удобнее, чем NULL
		})
		if err != nil {
			return fmt.Errorf("attach default role: %w", err)
		}
		created = u
		return nil
	})
	if err != nil {
		return nil, err
	}

	pair, err := s.tokens.Issue(ctx, pgutil.UUID(created.ID), ip)
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}
	return &Result{User: created, Pair: pair}, nil
}

// Login проверяет email+password и выдаёт пару токенов. Чтобы не
// раскрывать факт существования email, при ненайденном пользователе
// мы всё равно делаем bcrypt-сравнение с фиктивным хешем — это
// выравнивает время ответа.
func (s *Service) Login(ctx context.Context, email, password, ip string) (*Result, error) {
	emailLower := strings.ToLower(strings.TrimSpace(email))

	user, err := s.store.GetUserByEmail(ctx, emailLower)
	if err != nil {
		if repo.IsNotFound(err) {
			_ = checkPassword(dummyHash, password) // выравниваем время
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	if err := checkPassword(user.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	pair, err := s.tokens.Issue(ctx, pgutil.UUID(user.ID), ip)
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}
	return &Result{User: user, Pair: pair}, nil
}

// Refresh — прокси к TokenService.Rotate. Здесь нужен сервис auth
// в основном для единообразия (handler работает с одним сервисом).
func (s *Service) Refresh(ctx context.Context, refreshPlain, ip string) (*token.Pair, error) {
	return s.tokens.Rotate(ctx, refreshPlain, ip)
}

// Logout закрывает текущую сессию (точечный revoke access-токена).
func (s *Service) Logout(ctx context.Context, accessTokenID uuid.UUID) error {
	return s.tokens.Revoke(ctx, accessTokenID)
}

// Me возвращает профиль пользователя с ролями и плоским списком
// активных permission-slug'ов. Permission-список фронт может
// использовать для условного рендеринга кнопок.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	pgID := pgutil.PgUUID(userID)

	user, err := s.store.GetUserByID(ctx, pgID)
	if err != nil {
		if repo.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	roles, err := s.store.ListRolesForUser(ctx, pgID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	perms, err := s.store.ListPermissionsForUser(ctx, pgID)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	return &Profile{User: user, Roles: roles, Permissions: perms}, nil
}

func validateRegister(in RegisterInput) error {
	if !emailRegex.MatchString(strings.TrimSpace(in.Email)) {
		return ErrInvalidEmail
	}
	if len(in.Password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if strings.TrimSpace(in.FirstName) == "" || strings.TrimSpace(in.LastName) == "" {
		return fmt.Errorf("auth: first_name и last_name обязательны")
	}
	return nil
}

func dateOrNull(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}
