// Package timeutil централизует форматирование времени в часовом поясе
// вуза для человекочитаемых строк (письма, уведомления).
//
// Зачем: Postgres хранит timestamptz и pgx отдаёт его в UTC. Если
// форматировать напрямую через time.Time.Format, в письмо попадёт
// UTC-время, а не местное — пользователь увидит время на 5 часов раньше.
// Фронт отображает время в зоне вуза (Asia/Yekaterinburg, UTC+5) через
// Intl.DateTimeFormat, поэтому письма должны делать то же самое, иначе
// время в письме и в интерфейсе расходится.
package timeutil

import "time"

// UniversityTZ — IANA-имя часового пояса вуза (Ханты-Мансийск, UTC+5).
// Должно совпадать с UNIVERSITY_TZ на фронте (utils/datetime.js).
const UniversityTZ = "Asia/Yekaterinburg"

// universityLoc — загруженная зона вуза. Загружается один раз при старте.
// Если tzdata в окружении недоступна (минимальный образ без zoneinfo),
// LoadLocation вернёт ошибку — тогда используем фиксированное смещение
// +05:00 как безопасный fallback (у Ханты нет перехода на летнее время).
var universityLoc = mustUniversityLoc()

func mustUniversityLoc() *time.Location {
	if loc, err := time.LoadLocation(UniversityTZ); err == nil {
		return loc
	}
	return time.FixedZone("UTC+5", 5*60*60)
}

// FormatLocal форматирует момент времени в зоне вуза по layout.
func FormatLocal(t time.Time, layout string) string {
	return t.In(universityLoc).Format(layout)
}

// FormatDateTime — стандартный человекочитаемый формат "ГГГГ-ММ-ДД ЧЧ:ММ"
// в зоне вуза. Используется в email-уведомлениях о пересдачах.
func FormatDateTime(t time.Time) string {
	return FormatLocal(t, "2006-01-02 15:04")
}
