package scheduler

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// systemActorUUID — служебный actor для audit-записей из шедулера.
// uuid.Nil сюда не подходит, потому что audit_log.actor_id может
// быть NULL только если событие совсем без актёра — а у нас актёр
// есть, он системный.
func systemActorUUID() uuid.UUID {
	id, _ := uuid.Parse(SystemActorIDStr)
	return id
}

// uuidToString преобразует pgtype.UUID в каноническую строковую форму.
// Использует google/uuid, чтобы получить формат вида
// "00000000-0000-0000-0000-000000000000" а не hex-байты.
func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	u, err := uuid.FromBytes(id.Bytes[:])
	if err != nil {
		return ""
	}
	return u.String()
}
