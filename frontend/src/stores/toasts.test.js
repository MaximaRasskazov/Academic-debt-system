import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useToastsStore } from './toasts'

describe('useToastsStore', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('начальное состояние: items пустой', () => {
    const store = useToastsStore()
    expect(store.items).toHaveLength(0)
  })

  it('push добавляет тост с message и kind', () => {
    const store = useToastsStore()
    store.push('Тест', 'success')
    expect(store.items).toHaveLength(1)
    expect(store.items[0].message).toBe('Тест')
    expect(store.items[0].kind).toBe('success')
  })

  it('push использует info как kind по умолчанию', () => {
    const store = useToastsStore()
    store.push('Привет')
    expect(store.items[0].kind).toBe('info')
  })

  it('тост удаляется через 3 секунды', () => {
    const store = useToastsStore()
    store.push('Исчезну')
    expect(store.items).toHaveLength(1)
    vi.advanceTimersByTime(3000)
    expect(store.items).toHaveLength(0)
  })

  it('тост НЕ удаляется раньше 3 секунд', () => {
    const store = useToastsStore()
    store.push('Ещё здесь')
    vi.advanceTimersByTime(2999)
    expect(store.items).toHaveLength(1)
  })

  it('remove удаляет тост немедленно', () => {
    const store = useToastsStore()
    const id = store.push('Удали меня')
    store.remove(id)
    expect(store.items).toHaveLength(0)
  })

  it('два push создают два item с разными id', () => {
    const store = useToastsStore()
    store.push('A')
    store.push('B')
    expect(store.items).toHaveLength(2)
    expect(store.items[0].id).not.toBe(store.items[1].id)
  })
})
