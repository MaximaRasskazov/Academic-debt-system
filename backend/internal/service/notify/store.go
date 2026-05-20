package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/pgutil"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo"
	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/repo/queries"
)

type dbNotifier struct {
	store *repo.Store
}

func newDBNotifier(store *repo.Store) *dbNotifier {
	return &dbNotifier{store: store}
}

func (d *dbNotifier) persist(ctx context.Context, e Event) (Notification, error) {
	var payload []byte
	if e.Payload == nil {
		payload = []byte("{}")
	} else {
		var err error
		payload, err = json.Marshal(e.Payload)
		if err != nil {
			return Notification{}, fmt.Errorf("notify: marshal payload: %w", err)
		}
	}

	row, err := d.store.CreateNotification(ctx, queries.CreateNotificationParams{
		UserID:  pgutil.PgUUID(e.UserID),
		Kind:    e.Kind,
		Payload: payload,
	})
	if err != nil {
		return Notification{}, fmt.Errorf("notify: insert: %w", err)
	}
	return fromRow(row), nil
}
