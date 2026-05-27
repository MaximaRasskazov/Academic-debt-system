package report_test

// Тесты, не требующие БД — проверяют бизнес-логику до первого DB-вызова.
// store=nil намеренно: если до него доходит — тест упадёт с nil-деref,
// что немедленно укажет на регрессию в порядке проверок.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MaximaRasskazov/Academic-debt-system/backend/internal/service/report"
)

func TestReport_RetakesForPeriod_InvalidPeriod_NoDB(t *testing.T) {
	svc := report.New(nil) // store не нужен — проверка срабатывает раньше

	now := time.Now()
	cases := []struct {
		name string
		from time.Time
		to   time.Time
	}{
		{"from > to", now.Add(time.Hour), now},
		{"from == to", now, now},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.RetakesForPeriod(context.Background(), tc.from, tc.to)
			require.ErrorIs(t, err, report.ErrInvalidPeriod)
		})
	}
}
