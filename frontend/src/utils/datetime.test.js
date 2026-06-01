import { describe, it, expect } from 'vitest'
import { fmtTime, fmtDate, fmtDateTime, partsInTZ, toUtcISO } from './datetime'

// Ханты-Мансийск — UTC+5 (без перехода на летнее время).
// Ключевой инвариант бага: 21:00 по Ханты === 16:00 UTC.

describe('toUtcISO — ввод времени вуза → UTC', () => {
  it('21:00 в Ханты сохраняется как 16:00 UTC', () => {
    expect(toUtcISO('2026-06-01', '21:00')).toBe('2026-06-01T16:00:00.000Z')
  })

  it('00:30 в Ханты → 19:30 предыдущих суток UTC', () => {
    expect(toUtcISO('2026-06-01', '00:30')).toBe('2026-05-31T19:30:00.000Z')
  })

  it('независимо от зоны устройства даёт один и тот же UTC', () => {
    // toUtcISO не использует new Date("...без зоны"), поэтому не зависит
    // от TZ окружения — результат детерминирован.
    expect(toUtcISO('2026-12-31', '09:15')).toBe('2026-12-31T04:15:00.000Z')
  })
})

describe('fmtTime / partsInTZ — UTC → время вуза', () => {
  it('16:00 UTC показывается как 21:00 (Ханты)', () => {
    expect(fmtTime('2026-06-01T16:00:00.000Z')).toBe('21:00')
  })

  it('partsInTZ раскладывает UTC на компоненты в зоне вуза', () => {
    const p = partsInTZ('2026-06-01T16:00:00.000Z')
    expect(p).toMatchObject({ year: '2026', month: '06', day: '01', hour: '21', minute: '00' })
  })

  it('полночь по Ханты (19:00 UTC) → hour 00, дата следующая', () => {
    const p = partsInTZ('2026-06-01T19:00:00.000Z')
    expect(p.hour).toBe('00')
    expect(p.day).toBe('02')
  })
})

describe('round-trip ввод→хранение→вывод', () => {
  it('21:00 → UTC → обратно 21:00', () => {
    const utc = toUtcISO('2026-06-01', '21:00')
    expect(fmtTime(utc)).toBe('21:00')
    expect(fmtDate(utc, { day: '2-digit', month: '2-digit', year: 'numeric' })).toBe('01.06.2026')
  })
})

describe('fmtDateTime', () => {
  it('форматирует дату и время вместе в зоне вуза', () => {
    expect(fmtDateTime('2026-06-01T16:00:00.000Z')).toBe('01.06.2026, 21:00')
  })
})
