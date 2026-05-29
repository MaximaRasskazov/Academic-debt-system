import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useNotificationsStore } from './notifications'
import { notificationsApi } from '../api/notifications'

vi.mock('../api/notifications', () => ({
  notificationsApi: {
    unreadCount: vi.fn(),
    list:        vi.fn(),
    markRead:    vi.fn(),
  },
}))

// ── WS-заглушка ──────────────────────────────────────────────────────────────
let wsMock
let WsMockCtor

beforeEach(() => {
  vi.useFakeTimers()

  wsMock = {
    onopen: null, onmessage: null, onerror: null, onclose: null,
    close: vi.fn(),
    send:  vi.fn(),
    _open:    () => wsMock.onopen?.(),
    _message: (data) => wsMock.onmessage?.({ data: JSON.stringify(data) }),
    _close:   () => wsMock.onclose?.(),
  }
  WsMockCtor = vi.fn(() => wsMock)
  vi.stubGlobal('WebSocket', WsMockCtor)

  notificationsApi.unreadCount.mockResolvedValue({ count: 5 })
  notificationsApi.list.mockResolvedValue({
    items: [{ id: 'n1', kind: 'retake_scheduled', read_at: null }],
  })
  notificationsApi.markRead.mockResolvedValue({})
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

// ── Тесты ────────────────────────────────────────────────────────────────────
describe('useNotificationsStore', () => {
  it('начальное состояние: items=[], unreadCount=0', () => {
    const s = useNotificationsStore()
    expect(s.items).toHaveLength(0)
    expect(s.unreadCount).toBe(0)
  })

  it('loadInitial заполняет items и unreadCount из API', async () => {
    const s = useNotificationsStore()
    await s.loadInitial()
    expect(s.unreadCount).toBe(5)
    expect(s.items).toHaveLength(1)
    expect(s.items[0].id).toBe('n1')
  })

  it('loadInitial не падает при сетевой ошибке', async () => {
    notificationsApi.unreadCount.mockRejectedValue(new Error('net'))
    notificationsApi.list.mockRejectedValue(new Error('net'))
    const s = useNotificationsStore()
    await expect(s.loadInitial()).resolves.toBeUndefined()
    expect(s.items).toHaveLength(0)
  })

  it('ws.onmessage: непрочитанное → unreadCount++, item первым в списке', () => {
    const s = useNotificationsStore()
    s.connect('tok')
    wsMock._message({ id: 'n2', kind: 'retake_grade_received', read_at: null })
    expect(s.unreadCount).toBe(1)
    expect(s.items[0].id).toBe('n2')
  })

  it('ws.onmessage: уже прочитанное → unreadCount не меняется', () => {
    const s = useNotificationsStore()
    s.connect('tok')
    wsMock._message({ id: 'n3', kind: 'retake_updated', read_at: '2026-01-01T00:00:00Z' })
    expect(s.unreadCount).toBe(0)
    expect(s.items).toHaveLength(1)
  })

  it('markRead: optimistic unreadCount--, API вызван с нужным id', async () => {
    const s = useNotificationsStore()
    s.connect('tok')
    wsMock._message({ id: 'n4', kind: 'retake_scheduled', read_at: null })
    await s.markRead('n4')
    expect(s.unreadCount).toBe(0)
    expect(s.items[0].read_at).toBeTruthy()
    expect(notificationsApi.markRead).toHaveBeenCalledWith('n4')
  })

  it('markRead идемпотентен: повторный вызов не меняет unreadCount', async () => {
    const s = useNotificationsStore()
    s.connect('tok')
    wsMock._message({ id: 'n5', kind: 'retake_scheduled', read_at: null })
    await s.markRead('n5')
    await s.markRead('n5')
    expect(s.unreadCount).toBe(0)
    expect(notificationsApi.markRead).toHaveBeenCalledTimes(1)
  })

  it('disconnect: ws.close вызван, reconnect не происходит', () => {
    const s = useNotificationsStore()
    s.connect('tok')
    s.disconnect()
    expect(wsMock.close).toHaveBeenCalled()
    vi.advanceTimersByTime(60_000)
    expect(WsMockCtor).toHaveBeenCalledTimes(1)
  })

  it('exponential backoff: reconnect через retryDelay после разрыва', () => {
    const s = useNotificationsStore()
    s.connect('tok')
    expect(WsMockCtor).toHaveBeenCalledTimes(1)
    wsMock._close()
    expect(WsMockCtor).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(1_000)
    expect(WsMockCtor).toHaveBeenCalledTimes(2)
  })
})
