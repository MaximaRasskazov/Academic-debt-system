package timeutil

import (
	"testing"
	"time"
)

// TestFormatDateTime_ConvertsUTCToUniversityTZ проверяет, что UTC-момент
// форматируется во времени вуза (UTC+5), а не в UTC. Это ровно тот баг,
// из-за которого письмо показывало 04:00 вместо 09:00.
func TestFormatDateTime_ConvertsUTCToUniversityTZ(t *testing.T) {
	// 09:00 по времени вуза == 04:00 UTC.
	utc := time.Date(2026, 6, 5, 4, 0, 0, 0, time.UTC)
	got := FormatDateTime(utc)
	want := "2026-06-05 09:00"
	if got != want {
		t.Fatalf("FormatDateTime(%s) = %q, want %q", utc, got, want)
	}
}

// TestFormatLocal_RespectsLayout проверяет произвольный layout.
func TestFormatLocal_RespectsLayout(t *testing.T) {
	utc := time.Date(2026, 1, 2, 19, 30, 0, 0, time.UTC) // 00:30 след. суток в UTC+5
	got := FormatLocal(utc, "02.01.2006 15:04")
	want := "03.01.2026 00:30"
	if got != want {
		t.Fatalf("FormatLocal = %q, want %q", got, want)
	}
}
