// Package repo предоставляет фасад над сгенерированным sqlc-кодом из
// internal/repo/queries и реализует Unit of Work для атомарных мутаций.
//
// Сервисы получают *Store через DI и используют либо встроенный
// *queries.Queries для одиночных запросов, либо Store.RunInTx для
// последовательностей, требующих транзакции (мутация сущности + запись
// в change_logs / audit_log).
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

// Store — точка входа в слой доступа к данным.
//
// Встраивает *queries.Queries поверх пула соединений, поэтому все
// методы, сгенерированные sqlc, доступны напрямую как методы Store —
// например, store.GetUserByEmail(ctx, "u@example.com"). Внутри
// транзакции работа идёт через локальный *queries.Queries, который
// передаётся в RunInTx-callback.
type Store struct {
	*queries.Queries

	pool *pgxpool.Pool
}

// NewStore оборачивает уже открытый пул соединений.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		Queries: queries.New(pool),
		pool:    pool,
	}
}

// Pool возвращает нижележащий пул. Нужно тонкому кругу клиентов —
// например, healthcheck-у для Ping. Большинству сервисов pool не нужен.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// RunInTx выполняет fn внутри транзакции. Если fn возвращает ошибку
// (или сама транзакция не может закоммититься), изменения откатываются.
//
// Внутри fn нужно использовать переданный *queries.Queries — он
// прибинден к транзакции. Использование Store.Queries напрямую внутри
// fn создаст запись вне транзакции — это баг, не делать так.
//
// Пример:
//
//	err := store.RunInTx(ctx, func(q *queries.Queries) error {
//	    user, err := q.CreateUser(ctx, params)
//	    if err != nil { return err }
//	    _, err = q.CreateChangeLog(ctx, queries.CreateChangeLogParams{
//	        EntityType: "user", EntityID: user.ID.String(),
//	        Action: "created", After: afterJSON, CreatedBy: actorID,
//	    })
//	    return err
//	})
func (s *Store) RunInTx(ctx context.Context, fn func(*queries.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// Rollback на закрытой транзакции возвращает pgx.ErrTxClosed —
	// это нормальное поведение, ошибку игнорируем (она именно про
	// "транзакция уже завершилась успешным Commit").
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(queries.New(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// IsNotFound сообщает, является ли ошибка от sqlc-запроса
// "запись не найдена". pgx возвращает свою специфичную ошибку, и
// сервисы должны проверять её именно через этот хелпер, не импортируя
// pgx напрямую.
func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// DisciplineExists проверяет, существует ли дисциплина с данным id
// (включая soft-deleted записи). Нужно Restore, чтобы отличить
// "нет такой записи" от "запись активна и не требует восстановления".
func (s *Store) DisciplineExists(ctx context.Context, id pgtype.UUID) (bool, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM disciplines WHERE id = $1`, id).Scan(&n)
	return n > 0, err
}

// IsForeignKeyViolation сообщает, является ли err нарушением FOREIGN KEY
// (SQLSTATE 23503). Используется, чтобы отличить "referenced row not found"
// от других ошибок.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// GetLastSyncedAt возвращает метку времени последней успешной синхронизации.
func (s *Store) GetLastSyncedAt(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT last_synced_at FROM sync_state WHERE key = 'emulator'`,
	).Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("get last_synced_at: %w", err)
	}
	return t, nil
}

// SetLastSyncedAt обновляет метку времени последней успешной синхронизации.
func (s *Store) SetLastSyncedAt(ctx context.Context, t time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sync_state SET last_synced_at = $1 WHERE key = 'emulator'`,
		t,
	)
	if err != nil {
		return fmt.Errorf("set last_synced_at: %w", err)
	}
	return nil
}

// UpsertDisciplineFromSync вставляет или обновляет дисциплину по external_id.
// Используется SyncService для idempotent-синхронизации.
func (s *Store) UpsertDisciplineFromSync(ctx context.Context, p UpsertDisciplineParams) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO disciplines (name, code, description, external_id, source, created_by)
		VALUES ($1, $2, $3, $4, 'sync', $5)
		ON CONFLICT (external_id) WHERE external_id IS NOT NULL
		DO UPDATE SET
			name        = EXCLUDED.name,
			code        = EXCLUDED.code,
			description = EXCLUDED.description,
			updated_at  = NOW()
	`, p.Name, p.Code, p.Description, p.ExternalID, p.SystemUserID)
	if err != nil {
		return fmt.Errorf("upsert discipline: %w", err)
	}
	return nil
}

// UpsertDisciplineParams — параметры для UpsertDisciplineFromSync.
type UpsertDisciplineParams struct {
	Name         string
	Code         string
	Description  *string
	ExternalID   string
	SystemUserID pgtype.UUID
}

// UpsertDebtFromSync вставляет или обновляет долг по external_id.
// При конфликте обновляет только изменяемые поля (статус, оценка).
func (s *Store) UpsertDebtFromSync(ctx context.Context, p UpsertDebtParams) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO debts (student_id, discipline_id, issued_by, external_id, source, notes, status, final_grade, graded_at, graded_by)
		VALUES ($1, $2, $3, $4, 'sync', $5, $6, $7, $8, $9)
		ON CONFLICT (external_id) WHERE external_id IS NOT NULL
		DO UPDATE SET
			status      = EXCLUDED.status,
			final_grade = EXCLUDED.final_grade,
			graded_at   = EXCLUDED.graded_at,
			graded_by   = EXCLUDED.graded_by,
			notes       = EXCLUDED.notes,
			updated_at  = NOW()
	`, p.StudentID, p.DisciplineID, p.IssuedBy, p.ExternalID,
		p.Notes, p.Status, p.FinalGrade, p.GradedAt, p.GradedBy)
	if err != nil {
		return fmt.Errorf("upsert debt: %w", err)
	}
	return nil
}

// UpsertDebtParams — параметры для UpsertDebtFromSync.
type UpsertDebtParams struct {
	StudentID    pgtype.UUID
	DisciplineID pgtype.UUID
	IssuedBy     pgtype.UUID
	ExternalID   string
	Notes        *string
	Status       string
	FinalGrade   *int32
	GradedAt     pgtype.Timestamptz
	GradedBy     pgtype.UUID
}

// IsUniqueViolation сообщает, является ли err нарушением UNIQUE-ограничения
// (SQLSTATE 23505). Если constraintName непустой, дополнительно проверяется
// имя конкретного constraint/partial-индекса.
func IsUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return constraintName == "" || pgErr.ConstraintName == constraintName
}
