// Package emulator реализует HTTP-клиент для внешнего эмулятора деканата.
//
// Все запросы идут с заголовком X-API-Key. Клиент не содержит бизнес-логики —
// только транспортный слой: десериализация ответов, маппинг HTTP-ошибок.
package emulator

import (
	"context"
	"encoding/json"
	"fmt"
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
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ChangeEntry — одна запись из GET /api/v1/changes.
type ChangeEntry struct {
	ID         int64     `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Action     string    `json:"action"`
	OccurredAt time.Time `json:"occurred_at"`
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
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	MiddleName *string `json:"middle_name"`
	Role       string  `json:"role"`
	Status     string  `json:"status"`
}

// changesResponse — обёртка ответа GET /api/v1/changes.
type changesResponse struct {
	Items []ChangeEntry `json:"items"`
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

// GetChanges возвращает список изменений в эмуляторе, произошедших после since.
// Эмулятор принимает since в формате RFC3339.
func (c *Client) GetChanges(ctx context.Context, since time.Time, limit int) ([]ChangeEntry, error) {
	params := url.Values{}
	params.Set("since", since.UTC().Format(time.RFC3339))
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	var resp changesResponse
	if err := c.get(ctx, "/api/v1/changes?"+params.Encode(), &resp); err != nil {
		return nil, fmt.Errorf("get changes: %w", err)
	}
	return resp.Items, nil
}

// GetDiscipline возвращает дисциплину по внешнему ID.
func (c *Client) GetDiscipline(ctx context.Context, externalID string) (*DisciplineDTO, error) {
	var resp disciplineResponse
	if err := c.get(ctx, "/api/v1/disciplines/"+url.PathEscape(externalID), &resp); err != nil {
		return nil, fmt.Errorf("get discipline %s: %w", externalID, err)
	}
	return &resp.Data, nil
}

// GetDebt возвращает долг по внешнему ID.
func (c *Client) GetDebt(ctx context.Context, externalID string) (*DebtDTO, error) {
	var resp debtResponse
	if err := c.get(ctx, "/api/v1/debts/"+url.PathEscape(externalID), &resp); err != nil {
		return nil, fmt.Errorf("get debt %s: %w", externalID, err)
	}
	return &resp.Data, nil
}

// GetAccount возвращает аккаунт по внешнему ID.
func (c *Client) GetAccount(ctx context.Context, externalID string) (*AccountDTO, error) {
	var resp accountResponse
	if err := c.get(ctx, "/api/v1/accounts/"+url.PathEscape(externalID), &resp); err != nil {
		return nil, fmt.Errorf("get account %s: %w", externalID, err)
	}
	return &resp.Data, nil
}

// get выполняет GET-запрос, декодирует JSON в dest.
func (c *Client) get(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("emulator responded %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// ErrNotFound возвращается когда эмулятор ответил 404.
var ErrNotFound = fmt.Errorf("emulator: не найдено")
