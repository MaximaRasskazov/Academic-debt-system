// Package pgutil содержит мелкие хелперы для interop между pgx-типами
// (pgtype.*) и более привычными Go-типами (google/uuid, time.Time, ...).
//
// Назначение пакета — спрятать boilerplate конвертаций, чтобы сервисный
// слой не зависел напрямую от pgtype и не знал детали nullable-полей.
package pgutil

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// PgUUID оборачивает uuid.UUID в pgtype.UUID с Valid=true.
// uuid.Nil превращается в pgtype.UUID{Valid: false} — чтобы NULL в БД
// возникал из явного "пустого" значения, а не из забытого Valid-флага.
func PgUUID(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

// UUID возвращает uuid.UUID из pgtype.UUID. Если запись NULL — uuid.Nil.
func UUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

// PgString оборачивает *string в pgtype.Text. nil → NULL.
// Используется при апдейте опциональных полей через sqlc.narg.
func PgString(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
