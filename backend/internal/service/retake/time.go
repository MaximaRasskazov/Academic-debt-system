package retake

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgutilTime — обёртка над time.Time с явным Valid-флагом. Сервис
// принимает её, чтобы handler-слой мог передать "поле не задано"
// без указателя на time.Time. На границе с sqlc конвертируется в
// pgtype.Timestamptz.
//
// Делаем локальный тип, а не используем pgtype напрямую, чтобы
// handler не зависел от pgx-типов.
type pgutilTime struct {
	Time  time.Time
	Valid bool
}

// NewTime — конструктор из time.Time (Valid=true).
func NewTime(t time.Time) pgutilTime {
	return pgutilTime{Time: t, Valid: true}
}

// NewTimePtr возвращает *pgutilTime для опциональных полей PATCH.
// nil → не передаём в запрос (поле не меняется).
func NewTimePtr(t *time.Time) *pgutilTime {
	if t == nil {
		return nil
	}
	pt := NewTime(*t)
	return &pt
}

func (p pgutilTime) toTimestamptz() pgtype.Timestamptz {
	if !p.Valid {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: p.Time, Valid: true}
}
