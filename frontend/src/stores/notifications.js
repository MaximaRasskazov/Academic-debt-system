import { defineStore } from 'pinia'
import { ref } from 'vue'
import { notificationsApi } from '../api/notifications'
import { useToastsStore } from './toasts'

// Человекочитаемые метки для каждого kind — показываем в toast и dropdown.
const KIND_LABELS = {
  retake_scheduled:         'Назначена пересдача',
  retake_updated:           'Изменено расписание пересдачи',
  retake_cancelled:         'Пересдача отменена',
  retake_grade_received:    'Получена оценка за пересдачу',
  teacher_request_approved: 'Заявка на преподавателя одобрена',
  teacher_request_rejected: 'Заявка на преподавателя отклонена',
  retake_change_approved:   'Изменение пересдачи одобрено',
  retake_change_rejected:   'Изменение пересдачи отклонено',
}

export function kindLabel(kind) {
  return KIND_LABELS[kind] ?? 'Новое уведомление'
}

export const useNotificationsStore = defineStore('notifications', () => {
  const items       = ref([])
  const unreadCount = ref(0)

  let ws         = null
  let retryTimer = null
  let retryDelay = 1_000   // начинаем с 1с, удваиваем, потолок 30с
  let _token     = null    // запоминаем для reconnect

  // loadInitial — подтягиваем счётчик и последние 20 уведомлений при старте.
  // Promise.allSettled гарантирует, что один упавший запрос не ронает оба.
  async function loadInitial() {
    try {
      const [countRes, listRes] = await Promise.allSettled([
        notificationsApi.unreadCount(),
        notificationsApi.list({ limit: 20 }),
      ])
      if (countRes.status === 'fulfilled')
        unreadCount.value = countRes.value.count ?? 0
      if (listRes.status === 'fulfilled') {
        const d = listRes.value
        items.value = d.items ?? (Array.isArray(d) ? d : [])
      }
    } catch {
      // сеть недоступна при старте — не падаем, покажем когда откроется
    }
  }

  // connect — открывает WS. Вызывается при логине и после token refresh.
  // Закрывает предыдущее соединение, если было.
  function connect(token) {
    _token = token
    _closeWs()
    clearTimeout(retryTimer)
    if (!token) return

    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    // WS endpoint находится вне /api — прямой путь к бэку
    const host = import.meta.env.VITE_WS_HOST ?? location.host
    ws = new WebSocket(`${proto}://${host}/ws/notifications?token=${token}`)

    ws.onopen = () => {
      retryDelay = 1_000   // сбрасываем backoff при успешном коннекте
    }

    ws.onmessage = (e) => {
      try {
        const n = JSON.parse(e.data)
        items.value.unshift(n)
        if (!n.read_at) {
          unreadCount.value++
          useToastsStore().push(kindLabel(n.kind))
        }
      } catch {
        // некорректный JSON — игнорируем фрейм
      }
    }

    ws.onerror = () => {
      // onclose сработает следом — там reconnect
    }

    ws.onclose = () => {
      ws = null
      retryTimer = setTimeout(() => {
        retryDelay = Math.min(retryDelay * 2, 30_000)
        connect(_token)
      }, retryDelay)
    }
  }

  // disconnect — вызывается при logout. Гасит WS и reconnect-таймер.
  function disconnect() {
    _token = null
    clearTimeout(retryTimer)
    _closeWs()
  }

  // markRead — оптимистичное обновление + API-вызов. Идемпотентно.
  async function markRead(id) {
    const n = items.value.find((n) => n.id === id)
    if (!n || n.read_at) return
    n.read_at = new Date().toISOString()
    unreadCount.value = Math.max(0, unreadCount.value - 1)
    try {
      await notificationsApi.markRead(id)
    } catch {
      // best-effort: бэк идемпотентен, при следующем запросе состояние
      // синхронизируется
    }
  }

  function _closeWs() {
    if (ws) {
      ws.onclose = null   // отключаем reconnect-хендлер до принудительного close
      ws.close()
      ws = null
    }
  }

  return { items, unreadCount, loadInitial, connect, disconnect, markRead }
})
