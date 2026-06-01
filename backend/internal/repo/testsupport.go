package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CleanupUser удаляет пользователя вместе со всеми записями, которые на
// него ссылаются, в порядке внешних ключей. Предназначен для t.Cleanup
// в интеграционных тестах.
//
// Зачем нужен: на users висят FK с правилами RESTRICT / NO ACTION из
// audit_log, change_logs, debts, teacher_disciplines, disciplines и др.
// Наивный `DELETE FROM users WHERE id = $1` падает с FK-violation, если
// тест успел записать в эти таблицы (audit/changelog пишутся почти при
// каждой мутации). Раньше cleanup в тестах глотал эту ошибку через
// `_, _ = ...Exec(...)`, и тестовые пользователи копились в dev-БД
// бесконечно. Этот хелпер чистит зависимости явно, поэтому пользователь
// удаляется надёжно.
//
// Выполняется одной транзакцией: либо весь каскад удаляется, либо ничего
// (тогда вызывающий тест увидит ошибку, а не частично удалённые данные).
// Ошибку возвращаем, а не глушим — чтобы регрессия в схеме (новый FK на
// users) была видна сразу, а не приводила к молчаливому накоплению.
func CleanupUser(ctx context.Context, pool *pgxpool.Pool, userID pgtype.UUID) error {
	id := userID

	// Порядок важен: сначала таблицы с FK на users (RESTRICT/NO ACTION),
	// затем сами users. CASCADE-таблицы (access_tokens, notifications,
	// role_user.user_id, student/teacher_disciplines.*_id) уберутся
	// автоматически при удалении users — но teacher_disciplines.created_by
	// и disciplines.created_by это NO ACTION, поэтому чистим их явно.
	stmts := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM retake_participants WHERE retake_id IN (SELECT id FROM retakes WHERE created_by = $1 OR deleted_by = $1)`, []any{id}},
		{`DELETE FROM retake_participants WHERE user_id = $1 OR graded_by = $1`, []any{id}},
		{`DELETE FROM retake_change_requests WHERE requested_by = $1 OR reviewed_by = $1`, []any{id}},
		{`DELETE FROM retakes WHERE created_by = $1 OR deleted_by = $1`, []any{id}},
		{`DELETE FROM debts WHERE student_id = $1 OR issued_by = $1 OR graded_by = $1 OR deleted_by = $1`, []any{id}},
		{`DELETE FROM change_logs WHERE created_by = $1`, []any{id}},
		{`DELETE FROM audit_log WHERE actor_id = $1`, []any{id}},
		{`DELETE FROM teacher_role_requests WHERE requested_by = $1 OR reviewed_by = $1`, []any{id}},
		{`DELETE FROM teacher_disciplines WHERE teacher_id = $1 OR created_by = $1 OR deleted_by = $1`, []any{id}},
		{`DELETE FROM disciplines WHERE created_by = $1 OR deleted_by = $1`, []any{id}},
		{`DELETE FROM users WHERE id = $1`, []any{id}},
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cleanup user: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, s := range stmts {
		if _, err := tx.Exec(ctx, s.sql, s.args...); err != nil {
			return fmt.Errorf("cleanup user: %q: %w", s.sql, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cleanup user: commit: %w", err)
	}
	return nil
}

// CleanupAllTestUsers удаляет всех пользователей с email вида
// '%@test.local' и все зависящие от них записи. Предназначен для
// вызова из TestMain после m.Run() — это страховочная дочистка.
//
// Зачем нужна вдобавок к per-test CleanupUser: тесты разных пакетов
// делят одну БД и идут параллельно. Per-test cleanup убирает данные
// своего теста, но при гонках транзакций часть удалений может не
// пройти. TestMain-дочистка гарантирует, что после прогона пакета не
// остаётся тестового мусора — независимо от таймингов параллельных
// пакетов. Фильтр по '@test.local' не затрагивает dev-сиды и реальные
// (эмуляторные) данные.
//
// Ошибку возвращаем, чтобы TestMain мог её залогировать; падать на ней
// не обязательно (тесты уже прошли), но молча игнорировать не стоит.
func CleanupAllTestUsers(ctx context.Context, pool *pgxpool.Pool) error {
	const sel = `SELECT id FROM users WHERE email LIKE '%@test.local'`
	stmts := []string{
		`DELETE FROM retake_participants WHERE retake_id IN (SELECT id FROM retakes WHERE created_by IN (` + sel + `) OR deleted_by IN (` + sel + `))`,
		`DELETE FROM retake_participants WHERE user_id IN (` + sel + `) OR graded_by IN (` + sel + `)`,
		`DELETE FROM retake_change_requests WHERE requested_by IN (` + sel + `) OR reviewed_by IN (` + sel + `)`,
		`DELETE FROM retakes WHERE created_by IN (` + sel + `) OR deleted_by IN (` + sel + `)`,
		`DELETE FROM debts WHERE student_id IN (` + sel + `) OR issued_by IN (` + sel + `) OR graded_by IN (` + sel + `) OR deleted_by IN (` + sel + `)`,
		`DELETE FROM change_logs WHERE created_by IN (` + sel + `)`,
		`DELETE FROM audit_log WHERE actor_id IN (` + sel + `)`,
		`DELETE FROM teacher_role_requests WHERE requested_by IN (` + sel + `) OR reviewed_by IN (` + sel + `)`,
		`DELETE FROM teacher_disciplines WHERE teacher_id IN (` + sel + `) OR created_by IN (` + sel + `) OR deleted_by IN (` + sel + `)`,
		`DELETE FROM disciplines WHERE created_by IN (` + sel + `) OR deleted_by IN (` + sel + `)`,
		`DELETE FROM users WHERE email LIKE '%@test.local'`,
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cleanup all test users: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, q := range stmts {
		if _, err := tx.Exec(ctx, q); err != nil {
			return fmt.Errorf("cleanup all test users: %q: %w", q, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cleanup all test users: commit: %w", err)
	}
	return nil
}
