// Утилиты работы со временем в часовом поясе вуза (Ханты-Мансийск, UTC+5).
//
// Зачем: бэкенд хранит и отдаёт момент времени в UTC (ISO-строка с Z).
// Если форматировать его через стандартный new Date().getHours(), результат
// зависит от часового пояса устройства/окружения, где исполняется JS
// (браузер пользователя, SSR-контейнер в UTC и т.п.). Из-за этого время
// пересдачи «уезжало» на 5 часов. Поэтому всё отображение и ввод времени
// жёстко привязываем к зоне вуза — тогда все видят одинаковое время
// независимо от настроек своего устройства.

export const UNIVERSITY_TZ = 'Asia/Yekaterinburg' // UTC+5, Ханты-Мансийск

// fmtTime: ISO-строку (UTC) → "ЧЧ:ММ" в зоне вуза.
export function fmtTime(iso) {
  if (!iso) return ''
  return new Intl.DateTimeFormat('ru-RU', {
    timeZone: UNIVERSITY_TZ,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(iso))
}

// fmtDate: ISO-строку (UTC) → дата в зоне вуза. opts переопределяют формат
// (по умолчанию ДД месяц ГГГГ).
export function fmtDate(iso, opts = { day: '2-digit', month: 'long', year: 'numeric' }) {
  if (!iso) return '—'
  return new Intl.DateTimeFormat('ru-RU', { timeZone: UNIVERSITY_TZ, ...opts }).format(new Date(iso))
}

// fmtDateTime: ISO-строку (UTC) → "ДД.ММ.ГГГГ ЧЧ:ММ" в зоне вуза.
export function fmtDateTime(iso) {
  if (!iso) return '—'
  return new Intl.DateTimeFormat('ru-RU', {
    timeZone: UNIVERSITY_TZ,
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(iso))
}

// partsInTZ: разбирает ISO-строку (UTC) на компоненты в зоне вуза.
// Нужно, чтобы предзаполнить форму редактирования (часы/минуты) тем же
// временем, что видит пользователь, а не временем устройства.
export function partsInTZ(iso) {
  const d = iso ? new Date(iso) : new Date()
  const p = new Intl.DateTimeFormat('en-CA', {
    timeZone: UNIVERSITY_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(d)
  const get = (t) => p.find((x) => x.type === t)?.value || ''
  return {
    year: get('year'),
    month: get('month'),
    day: get('day'),
    // Intl может вернуть "24" для полуночи — нормализуем к "00".
    hour: get('hour') === '24' ? '00' : get('hour'),
    minute: get('minute'),
  }
}

// toUtcISO: дата (ГГГГ-ММ-ДД) + время (ЧЧ:ММ), введённые пользователем
// КАК ВРЕМЯ ВУЗА, → ISO-строка в UTC для отправки на бэкенд.
//
// Нельзя полагаться на new Date("YYYY-MM-DDTHH:mm") — он интерпретирует
// строку в зоне устройства. Поэтому вычисляем смещение зоны вуза для
// конкретной даты и вычитаем его вручную (корректно и для перехода на
// летнее время, если бы он был — у Ханты его нет, но подход общий).
export function toUtcISO(dateStr, timeStr) {
  const [y, mo, d] = dateStr.split('-').map(Number)
  const [h, mi] = timeStr.split(':').map(Number)
  // Момент, как если бы введённые числа были в UTC.
  const asUtc = Date.UTC(y, mo - 1, d, h, mi, 0)
  // На сколько зона вуза опережает UTC именно для этого момента (мс).
  const offsetMs = tzOffsetMs(UNIVERSITY_TZ, new Date(asUtc))
  // Реальный UTC-момент = (числа как UTC) − смещение зоны.
  return new Date(asUtc - offsetMs).toISOString()
}

// tzOffsetMs: смещение указанной зоны относительно UTC для данного момента
// (в миллисекундах, положительное для зон восточнее UTC).
function tzOffsetMs(timeZone, date) {
  // Берём «стенные часы» зоны для этого UTC-момента и сравниваем с UTC.
  const dtf = new Intl.DateTimeFormat('en-US', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
  const p = Object.fromEntries(dtf.formatToParts(date).map((x) => [x.type, x.value]))
  const wall = Date.UTC(
    Number(p.year),
    Number(p.month) - 1,
    Number(p.day),
    p.hour === '24' ? 0 : Number(p.hour),
    Number(p.minute),
    Number(p.second),
  )
  return wall - date.getTime()
}
