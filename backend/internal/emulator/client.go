// Package emulator реализует HTTP-клиент для внешнего эмулятора деканата.
//
// Все запросы идут с заголовком X-API-Key. Клиент не содержит бизнес-логики —
// только транспортный слой: десериализация ответов, маппинг HTTP-ошибок.
package emulator

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client отправляет запросы к эмулятору деканата.
type Client struct {
	base   string
	apiKey string
	http   *http.Client
}

// New создаёт клиент. baseURL — адрес эмулятора без trailing slash,
// например "https://hant-decanat.freydin.space".
func New(baseURL, apiKey string) *Client {
	return &Client{
		base:   baseURL,
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// Сервер эмулятора запрашивает TLS renegotiation;
				// Go по умолчанию не поддерживает его — соединение зависает.
				TLSClientConfig: &tls.Config{ //nolint:gosec
					Renegotiation: tls.RenegotiateOnceAsClient,
				},
			},
		},
	}
}

// ChangeEntry — одна запись из GET /api/v1/changes.
type ChangeEntry struct {
	ID         string         `json:"id"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Action     string         `json:"action"`
	OccurredAt time.Time      `json:"occurred_at"`
	NewValue   map[string]any `json:"new_value"`
}

// DisciplineDTO — данные дисциплины из эмулятора.
type DisciplineDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Description  *string `json:"description"`
	Department   *string `json:"department"`
	Semester     *int    `json:"semester"`
	AcademicYear *string `json:"academic_year"`
}

// DebtDTO — данные долга из эмулятора.
type DebtDTO struct {
	ID           string  `json:"id"`
	StudentID    string  `json:"student_id"`
	DisciplineID string  `json:"discipline_id"`
	TeacherID    string  `json:"teacher_id"`
	Status       string  `json:"status"`
	Grade        *int    `json:"grade"`
	Notes        *string `json:"notes"`
}

// AccountDTO — данные аккаунта из эмулятора.
type AccountDTO struct {
	ID             string  `json:"id"`
	Email          string  `json:"email"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	MiddleName     *string `json:"middle_name"`
	Role           string  `json:"role"`
	Status         string  `json:"status"`
	PasswordHash   string  `json:"password_hash"`
	LinkedEntityID string  `json:"linked_entity_id"` // ID студента/преподавателя (используется в долгах)
}

// debtsListResponse — обёртка ответа GET /api/v1/debts (список).
type debtsListResponse struct {
	Data []DebtDTO      `json:"data"`
	Meta paginationMeta `json:"meta"`
}

// changesResponse — обёртка ответа GET /api/v1/changes.
type changesResponse struct {
	Data []ChangeEntry `json:"data"`
	Meta changesMeta   `json:"meta"`
}

type changesMeta struct {
	NextSince string `json:"next_since"`
	HasMore   bool   `json:"has_more"`
}

// disciplinesListResponse — обёртка ответа GET /api/v1/disciplines (список).
type disciplinesListResponse struct {
	Data []DisciplineDTO `json:"data"`
	Meta paginationMeta  `json:"meta"`
}

type paginationMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// disciplineResponse — обёртка ответа GET /api/v1/disciplines/:id.
type disciplineResponse struct {
	Data DisciplineDTO `json:"data"`
}

// debtResponse — обёртка ответа GET /api/v1/debts/:id.
type debtResponse struct {
	Data DebtDTO `json:"data"`
}

// accountResponse — обёртка ответа GET /api/v1/accounts/:id.
type accountResponse struct {
	Data AccountDTO `json:"data"`
}

// accountsListResponse — обёртка ответа GET /api/v1/accounts (список).
type accountsListResponse struct {
	Data []AccountDTO   `json:"data"`
	Meta paginationMeta `json:"meta"`
}

// ChangesPage — одна страница изменений из эмулятора.
type ChangesPage struct {
	Entries   []ChangeEntry
	NextSince string // курсор для следующей страницы (пустой если HasMore=false)
	HasMore   bool
}

// GetChangesPage возвращает одну страницу изменений. Используйте NextSince
// для получения следующей страницы пока HasMore = true.
func (c *Client) GetChangesPage(ctx context.Context, since time.Time, limit int) (ChangesPage, error) {
	params := url.Values{}
	params.Set("since", since.UTC().Format(time.RFC3339))
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	var resp changesResponse
	if err := c.get(ctx, "/api/v1/changes?"+params.Encode(), &resp); err != nil {
		return ChangesPage{}, fmt.Errorf("get changes: %w", err)
	}
	return ChangesPage{
		Entries:   resp.Data,
		NextSince: resp.Meta.NextSince,
		HasMore:   resp.Meta.HasMore,
	}, nil
}

// GetChanges возвращает список изменений в эмуляторе, произошедших после since.
// Устарел: используйте GetChangesPage для корректной обработки has_more.
func (c *Client) GetChanges(ctx context.Context, since time.Time, limit int) ([]ChangeEntry, error) {
	page, err := c.GetChangesPage(ctx, since, limit)
	if err != nil {
		return nil, err
	}
	return page.Entries, nil
}

// ListDisciplines возвращает все дисциплины из эмулятора постранично.
func (c *Client) ListDisciplines(ctx context.Context, page, limit int) ([]DisciplineDTO, paginationMeta, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("limit", fmt.Sprintf("%d", limit))

	var resp disciplinesListResponse
	if err := c.get(ctx, "/api/v1/disciplines?"+params.Encode(), &resp); err != nil {
		return nil, paginationMeta{}, fmt.Errorf("list disciplines: %w", err)
	}
	return resp.Data, resp.Meta, nil
}

// GetDiscipline возвращает дисциплину по внешнему ID.
func (c *Client) GetDiscipline(ctx context.Context, externalID string) (*DisciplineDTO, error) {
	var resp disciplineResponse
	if err := c.get(ctx, "/api/v1/disciplines/"+url.PathEscape(externalID), &resp); err != nil {
		return nil, fmt.Errorf("get discipline %s: %w", externalID, err)
	}
	return &resp.Data, nil
}

// ListDebts возвращает долги из эмулятора постранично.
func (c *Client) ListDebts(ctx context.Context, page, limit int) ([]DebtDTO, paginationMeta, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("limit", fmt.Sprintf("%d", limit))

	var resp debtsListResponse
	if err := c.get(ctx, "/api/v1/debts?"+params.Encode(), &resp); err != nil {
		return nil, paginationMeta{}, fmt.Errorf("list debts: %w", err)
	}
	return resp.Data, resp.Meta, nil
}

// GetDebt возвращает долг по внешнему ID.
func (c *Client) GetDebt(ctx context.Context, externalID string) (*DebtDTO, error) {
	var resp debtResponse
	if err := c.get(ctx, "/api/v1/debts/"+url.PathEscape(externalID), &resp); err != nil {
		return nil, fmt.Errorf("get debt %s: %w", externalID, err)
	}
	return &resp.Data, nil
}

// ListAccounts возвращает аккаунты из эмулятора постранично.
// role: "student" | "teacher" | "" (все роли).
func (c *Client) ListAccounts(ctx context.Context, role string, page, limit int) ([]AccountDTO, paginationMeta, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("limit", fmt.Sprintf("%d", limit))
	if role != "" {
		params.Set("role", role)
	}

	var resp accountsListResponse
	if err := c.get(ctx, "/api/v1/accounts?"+params.Encode(), &resp); err != nil {
		return nil, paginationMeta{}, fmt.Errorf("list accounts: %w", err)
	}
	return resp.Data, resp.Meta, nil
}

// GetAccount возвращает аккаунт по внешнему ID.
func (c *Client) GetAccount(ctx context.Context, externalID string) (*AccountDTO, error) {
	var resp accountResponse
	if err := c.get(ctx, "/api/v1/accounts/"+url.PathEscape(externalID), &resp); err != nil {
		return nil, fmt.Errorf("get account %s: %w", externalID, err)
	}
	return &resp.Data, nil
}

// get выполняет GET-запрос с backoff на 429-ответы.
// Эмулятор при 429 сообщает retry_after_seconds=30, поэтому ждём 35с.
// RetryAfter — пауза между повторными попытками после 429.
// В проде эмулятор просит retry_after_seconds=30, ставим 35 с запасом.
// Тесты могут перебить на короткий интервал (см. service_test.go) чтобы
// не ждать минуты на покрытие rate-limit-сценария.
var RetryAfter = 35 * time.Second

// MaxAttempts — сколько раз пробуем при 429 перед тем как вернуть
// ErrRateLimited. Тоже переменная-пакета для тестов.
var MaxAttempts = 5

func (c *Client) get(ctx context.Context, path string, dest any) error {
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		done, err := c.doRequest(ctx, path, dest)
		if done {
			return err
		}
		if attempt == MaxAttempts {
			return ErrRateLimited
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(RetryAfter):
		}
	}
	return ErrRateLimited
}

// doRequest выполняет один HTTP-запрос. Возвращает done=true, если
// результат финальный (успех или ошибка кроме 429); done=false если
// получили 429 и стоит повторить.
func (c *Client) doRequest(ctx context.Context, path string, dest any) (done bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return true, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return true, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return true, ErrNotFound
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		// 429 → caller сделает backoff и повторит.
		return false, nil
	}
	if resp.StatusCode >= 400 {
		return true, fmt.Errorf("emulator responded %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return true, fmt.Errorf("decode response: %w", err)
	}
	return true, nil
}

// ErrNotFound возвращается когда эмулятор ответил 404.
var ErrNotFound = fmt.Errorf("emulator: не найдено")

// ErrRateLimited возвращается, когда эмулятор ответил 429 на все попытки
// retry. Sync-сервис должен ловить эту ошибку и НЕ сдвигать
// last_synced_at, чтобы при следующем тике эти change'ы пришли снова.
var ErrRateLimited = fmt.Errorf("emulator: rate limited (429)")

// authCheckRequest — тело POST /api/v1/auth/check.
type authCheckRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// authCheckResponse — успешный ответ auth/check (200).
// Нас интересует только подтверждение и external_id для связки
// с локальной записью пользователя.
type authCheckResponse struct {
	Success bool `json:"success"`
	Account struct {
		ExternalID string `json:"external_id"`
		Email      string `json:"email"`
		Role       string `json:"role"`
	} `json:"account"`
}

// ErrAuthInvalid возвращается, когда эмулятор ответил 401 —
// учётные данные неверны. Отличается от ErrRateLimited и сетевых
// ошибок, чтобы auth-слой мог различить «неверный пароль» и
// «эмулятор недоступен».
var ErrAuthInvalid = fmt.Errorf("emulator: неверные учётные данные")

// AuthCheck проверяет email+password на стороне эмулятора.
// Возвращает external_id подтверждённого аккаунта при успехе,
// ErrAuthInvalid при 401, или сетевую/прочую ошибку если эмулятор
// недоступен (auth-слой трактует это как «сервис деканата лёг»).
func (c *Client) AuthCheck(ctx context.Context, email, password string) (string, error) {
	body, err := json.Marshal(authCheckRequest{Email: email, Password: password})
	if err != nil {
		return "", fmt.Errorf("marshal auth check: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.base+"/api/v1/auth/check", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build auth check request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("do auth check: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrAuthInvalid
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth check: эмулятор ответил %d", resp.StatusCode)
	}

	var out authCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode auth check: %w", err)
	}
	if !out.Success || out.Account.ExternalID == "" {
		return "", ErrAuthInvalid
	}
	return out.Account.ExternalID, nil
}

// Grade — оценка для отправки в эмулятор.
// Type: "numeric" (Value — int) или "pass_fail" (Value — "passed"/"failed").
type Grade struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// ErrGradeRejected возвращается, когда эмулятор ответил 422 —
// валидация тела не прошла. Это сигнал о баге маппинга на нашей
// стороне, а не «эмулятор недоступен», поэтому ошибка отдельная.
var ErrGradeRejected = fmt.Errorf("emulator: оценка отклонена валидацией")

// debtGradeRequest — тело PATCH /api/v1/debts/{id}/grade.
type debtGradeRequest struct {
	Grade          Grade   `json:"grade"`
	Comment        *string `json:"comment"`
	IdempotencyKey string  `json:"idempotency_key"`
}

// PatchDebtGrade закрывает долг в эмуляторе, проставляя оценку.
// debtExternalID — идентификатор долга на стороне эмулятора (UUID,
// который sync сохранил в debts.external_id). idempotencyKey —
// стабильный ключ для защиты от двойной отправки.
//
// Ответы: 200 → nil; 422 → ErrGradeRejected (с reason в тексте для
// лога); 404 → ErrNotFound; иной код/сетевой сбой → обычная ошибка
// (трактуется вызывающим как «эмулятор недоступен»).
func (c *Client) PatchDebtGrade(ctx context.Context, debtExternalID string, grade Grade, comment *string, idempotencyKey string) error {
	body, err := json.Marshal(debtGradeRequest{
		Grade:          grade,
		Comment:        comment,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("marshal grade request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
		c.base+"/api/v1/debts/"+url.PathEscape(debtExternalID)+"/grade", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build patch grade request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do patch grade: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusUnprocessableEntity:
		// 422 — валидация; вытаскиваем reason в обёртку для лога.
		reason := readErrorReason(resp.Body)
		if reason != "" {
			return fmt.Errorf("%w: %s", ErrGradeRejected, reason)
		}
		return ErrGradeRejected
	default:
		return fmt.Errorf("patch grade: эмулятор ответил %d", resp.StatusCode)
	}
}

// readErrorReason пытается достать человекочитаемую причину из тела
// ошибки эмулятора ({"error":{"details":{"reason":"..."}}}). Любая
// проблема разбора — пустая строка (для лога это не критично).
func readErrorReason(body io.Reader) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Details struct {
				Field  string `json:"field"`
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(body).Decode(&e); err != nil {
		return ""
	}
	if e.Error.Details.Reason != "" {
		if e.Error.Details.Field != "" {
			return e.Error.Details.Field + ": " + e.Error.Details.Reason
		}
		return e.Error.Details.Reason
	}
	return e.Error.Message
}
