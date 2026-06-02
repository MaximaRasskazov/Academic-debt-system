<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'
import StatCard from '../components/StatCard.vue'
import EmptyPlate from '../components/EmptyPlate.vue'
import { useAuthStore } from '../stores/auth'
import { debtsApi } from '../api/debts'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { notificationsApi } from '../api/notifications'
import { notifMeta, notifTitle, notifBody, timeAgo } from '../utils/notifFormat'
import { exportExcel, exportWord } from '../utils/exportTable'
import { partsInTZ } from '../utils/datetime'

const auth = useAuthStore()
const router = useRouter()
const sidebarOpen = ref(false)

function goToRetake(n) {
  const retakeId = n.payload?.retake_id
  if (!retakeId) return
  markRead(n)
  router.push({ path: '/retakes', query: { retake_id: retakeId } })
}

// ── Stats ─────────────────────────────────────────────────
const upcomingRetakes = ref([])

// Сырые данные для панелей
const myDebts    = ref([])   // все долги
const myDiscs    = ref([])   // [{id, name, code}]
const myRetakes  = ref([])
const discMap    = ref({})

const stats = ref([
  { key: 'debts',   label: 'Мои долги',           value: '—', accent: '#e63c5a' },
  { key: 'discs',   label: 'Мои дисциплины',      value: '—', accent: '#3b3fe0' },
  { key: 'retakes', label: 'Назначено пересдач',  value: '—', accent: '#f59e0b' },
  { key: 'done',    label: 'Завершённые пересдачи', value: '—', accent: '#10b981' },
])

// ── Active panel ──────────────────────────────────────────
// Раскрываемые панели: долги / дисциплины / назначенные / завершённые пересдачи
const PANEL_KEYS = ['debts', 'discs', 'retakes', 'done']
const activePanel = ref(null)

function togglePanel(key) {
  if (!PANEL_KEYS.includes(key)) return
  activePanel.value = activePanel.value === key ? null : key
}

// ── Производные списки для панелей ────────────────────────
// Долги: без закрытых (graded). Показываем актуальные/отменённые.
const openDebtsList = computed(() => myDebts.value.filter(d => d.status !== 'graded'))
// Назначенные пересдачи: без завершённых.
const scheduledRetakesList = computed(() => myRetakes.value.filter(r => r.status === 'scheduled' || r.status === 'in_progress'))
// Завершённые пересдачи.
const completedRetakesList = computed(() => myRetakes.value.filter(r => r.status === 'completed'))

// ── Feed ──────────────────────────────────────────────────
const feed         = ref([])
const feedLoading  = ref(true)
const unreadCount  = ref(0)

async function markRead(n) {
  if (n.read_at) return
  n.read_at = new Date().toISOString()
  unreadCount.value = Math.max(0, unreadCount.value - 1)
  try { await notificationsApi.markRead(n.id) } catch { /* silent */ }
}

async function markAllRead() {
  const unread = feed.value.filter(n => !n.read_at)
  unread.forEach(n => { n.read_at = new Date().toISOString() })
  unreadCount.value = 0
  await Promise.allSettled(unread.map(n => notificationsApi.markRead(n.id)))
}

// ── WebSocket ─────────────────────────────────────────────
let ws = null
function connectWS() {
  if (!auth.token) return
  try {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    ws = new WebSocket(`${proto}://${location.host}/ws/notifications?token=${auth.token}`)
    ws.onmessage = (e) => {
      try {
        const n = JSON.parse(e.data)
        feed.value.unshift(n)
        if (!n.read_at) unreadCount.value++
      } catch { /* ignore non-json */ }
    }
    ws.onerror = () => {}
  } catch { /* ignore ws errors */ }
}

// ── Helpers ───────────────────────────────────────────────
function statusLabel(s) {
  return { open: 'Открыт', graded: 'Закрыт', cancelled: 'Отменён' }[s] || s
}
function retakeStatusLabel(s) {
  return { scheduled: 'Назначена', in_progress: 'Идёт', completed: 'Завершена', cancelled: 'Отменена' }[s] || s
}
function fmtDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}
function fmtTime(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return `${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`
}

// ── Load ──────────────────────────────────────────────────
onMounted(async () => {
  connectWS()

  const [debtsRes, retakesRes, discRes, notifsRes] = await Promise.allSettled([
    debtsApi.getMy(),
    retakesApi.getMy(),
    disciplinesApi.myAsStudent(),
    notificationsApi.getAll({ limit: 30 }),
  ])

  myDiscs.value = discRes.status === 'fulfilled'
    ? (discRes.value.data.items ?? discRes.value.data ?? [])
    : []
  myDiscs.value.forEach(d => { discMap.value[d.id] = d.name || d.code })

  myDebts.value = debtsRes.status === 'fulfilled'
    ? (debtsRes.value.data.items ?? debtsRes.value.data ?? [])
    : []
  const openDebts = myDebts.value.filter(d => d.status === 'open').length

  let scheduledCount = 0, completedCount = 0
  if (retakesRes.status === 'fulfilled') {
    const items = retakesRes.value.data.items ?? retakesRes.value.data ?? []
    myRetakes.value = items
    scheduledCount = items.filter(r => r.status === 'scheduled' || r.status === 'in_progress').length
    completedCount = items.filter(r => r.status === 'completed').length
    upcomingRetakes.value = items
      .filter(r => r.status === 'scheduled')
      .slice(0, 10)
      .map(r => {
        // Время вуза (Ханты, UTC+5), а не зона устройства.
        const p = partsInTZ(r.scheduled_at)
        return {
          id: r.id,
          subject: discMap.value[r.discipline_id] || 'Дисциплина',
          day: Number(p.day), month: Number(p.month), year: Number(p.year),
          time: `${p.hour}:${p.minute}`,
          building: r.building, room: r.room,
        }
      })
  }

  if (notifsRes.status === 'fulfilled') {
    feed.value = notifsRes.value.data.items ?? notifsRes.value.data ?? []
    unreadCount.value = feed.value.filter(n => !n.read_at).length
  }
  feedLoading.value = false

  stats.value = [
    { key: 'debts',   label: 'Мои долги',            value: openDebts,           accent: '#e63c5a' },
    { key: 'discs',   label: 'Мои дисциплины',       value: myDiscs.value.length, accent: '#3b3fe0' },
    { key: 'retakes', label: 'Назначено пересдач',   value: scheduledCount,      accent: '#f59e0b' },
    { key: 'done',    label: 'Завершённые пересдачи', value: completedCount,     accent: '#10b981' },
  ]
})

function exportDebts(fmt) {
  const headers = ['Дисциплина', 'Статус', 'Оценка', 'Дата создания']
  const rows = openDebtsList.value.map(d => [
    discMap.value[d.discipline_id] || d.discipline_id,
    statusLabel(d.status),
    d.final_grade ?? '—',
    formatDate(d.created_at),
  ])
  const fn = fmt === 'excel' ? exportExcel : exportWord
  fn('Мои академические долги', headers, rows, 'moi_dolgi')
}

function formatDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU')
}

onUnmounted(() => { ws?.close() })
</script>

<template>
  <div class="student-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-student">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">

        <!-- ── Content columns ── -->
        <section class="content-cols">

          <!-- Left: stats + panels + feed -->
          <div class="feed-col">

            <!-- Stats -->
            <div class="stats-row">
              <StatCard
                v-for="s in stats" :key="s.key"
                :label="s.label"
                :value="s.value"
                clickable
                :active="activePanel === s.key"
                @click="togglePanel(s.key)"
              />
            </div>

            <!-- Panel: Мои долги (без закрытых) -->
            <Transition name="panel">
              <div v-if="activePanel === 'debts'" class="panel-card">
                <div class="panel-head">
                  <h3 class="panel-title">Мои долги</h3>
                  <div class="panel-head-actions">
                    <button class="btn-exp excel" :disabled="openDebtsList.length === 0" @click="exportDebts('excel')">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/></svg>
                      Excel
                    </button>
                    <button class="btn-exp word" :disabled="openDebtsList.length === 0" @click="exportDebts('word')">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M9 13l2 4 2-4 2 4"/></svg>
                      Word
                    </button>
                    <button class="panel-close" @click="activePanel = null">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                    </button>
                  </div>
                </div>
                <EmptyPlate v-if="openDebtsList.length === 0" icon="check" text="Долгов нет" />
                <ul v-else class="panel-list">
                  <li v-for="d in openDebtsList" :key="d.id" :class="['panel-row', `row-${d.status}`]">
                    <div class="panel-row-main">
                      <span class="panel-disc">{{ discMap[d.discipline_id] || d.discipline_id }}</span>
                      <span :class="['panel-badge', `badge-${d.status}`]">{{ statusLabel(d.status) }}</span>
                    </div>
                    <div class="panel-row-meta">
                      <span v-if="d.final_grade" class="panel-grade">Оценка: {{ d.final_grade }}</span>
                      <span class="panel-date">{{ fmtDate(d.created_at) }}</span>
                    </div>
                  </li>
                </ul>
              </div>
            </Transition>

            <!-- Panel: Мои дисциплины (здесь можно показывать статус «Закрыт») -->
            <Transition name="panel">
              <div v-if="activePanel === 'discs'" class="panel-card">
                <div class="panel-head">
                  <h3 class="panel-title">Мои дисциплины</h3>
                  <button class="panel-close" @click="activePanel = null">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                  </button>
                </div>
                <EmptyPlate v-if="myDiscs.length === 0" icon="book" text="Дисциплин нет" />
                <ul v-else class="panel-list">
                  <li v-for="d in myDiscs" :key="d.id" class="panel-row panel-row--disc">
                    <div class="disc-icon">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M2 3h6a4 4 0 014 4v14a3 3 0 00-3-3H2z"/><path d="M22 3h-6a4 4 0 00-4 4v14a3 3 0 013-3h7z"/></svg>
                    </div>
                    <div class="disc-body">
                      <span class="disc-name">{{ d.name }}</span>
                      <span v-if="d.code" class="disc-code">{{ d.code }}</span>
                    </div>
                  </li>
                </ul>
              </div>
            </Transition>

            <!-- Panel: Назначенные пересдачи (без завершённых) -->
            <Transition name="panel">
              <div v-if="activePanel === 'retakes'" class="panel-card">
                <div class="panel-head">
                  <h3 class="panel-title">Назначенные пересдачи</h3>
                  <button class="panel-close" @click="activePanel = null">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                  </button>
                </div>
                <EmptyPlate v-if="scheduledRetakesList.length === 0" icon="calendar" text="Назначенных пересдач нет" />
                <ul v-else class="panel-list">
                  <li v-for="r in scheduledRetakesList" :key="r.id" :class="['panel-row', `row-retake-${r.status}`]">
                    <div class="panel-row-main">
                      <span class="panel-disc">{{ discMap[r.discipline_id] || r.discipline_id }}</span>
                      <span :class="['panel-badge', `badge-retake-${r.status}`]">{{ retakeStatusLabel(r.status) }}</span>
                    </div>
                    <div class="panel-row-meta">
                      <span class="panel-date">{{ fmtDate(r.scheduled_at) }} {{ fmtTime(r.scheduled_at) }}</span>
                      <span v-if="r.building || r.room" class="panel-room">{{ r.building ? `корп. ${r.building}` : '' }}{{ r.room ? ` ауд. ${r.room}` : '' }}</span>
                    </div>
                  </li>
                </ul>
              </div>
            </Transition>

            <!-- Panel: Завершённые пересдачи -->
            <Transition name="panel">
              <div v-if="activePanel === 'done'" class="panel-card">
                <div class="panel-head">
                  <h3 class="panel-title">Завершённые пересдачи</h3>
                  <button class="panel-close" @click="activePanel = null">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                  </button>
                </div>
                <EmptyPlate v-if="completedRetakesList.length === 0" icon="calendar" text="Завершённых пересдач нет" />
                <ul v-else class="panel-list">
                  <li v-for="r in completedRetakesList" :key="r.id" class="panel-row row-retake-completed">
                    <div class="panel-row-main">
                      <span class="panel-disc">{{ discMap[r.discipline_id] || r.discipline_id }}</span>
                      <span class="panel-badge badge-retake-completed">{{ retakeStatusLabel(r.status) }}</span>
                    </div>
                    <div class="panel-row-meta">
                      <span class="panel-date">{{ fmtDate(r.scheduled_at) }} {{ fmtTime(r.scheduled_at) }}</span>
                      <span v-if="r.building || r.room" class="panel-room">{{ r.building ? `корп. ${r.building}` : '' }}{{ r.room ? ` ауд. ${r.room}` : '' }}</span>
                    </div>
                  </li>
                </ul>
              </div>
            </Transition>

            <!-- Feed -->
            <div class="feed-card">
              <div class="feed-head">
                <div class="feed-title-row">
                  <h2 class="feed-title">Лента событий</h2>
                  <span v-if="unreadCount > 0" class="unread-badge">{{ unreadCount }}</span>
                </div>
                <button v-if="unreadCount > 0" class="mark-all-btn" @click="markAllRead">
                  Прочитать все
                </button>
              </div>

              <div v-if="feedLoading" class="feed-state">
                <div class="spinner" />
                <span>Загрузка…</span>
              </div>

              <div v-else-if="feed.length === 0" class="feed-state feed-empty">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                  <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 01-3.46 0"/>
                </svg>
                <span>Уведомлений пока нет</span>
              </div>

              <ul v-else class="feed-list">
                <li
                  v-for="n in feed" :key="n.id"
                  class="feed-item"
                  :class="{ unread: !n.read_at }"
                  @click="markRead(n)"
                >
                  <div class="notif-icon-wrap" :style="{ background: notifMeta(n.kind ?? n.type).bg }">
                    <img v-if="notifMeta(n.kind ?? n.type).img" :src="notifMeta(n.kind ?? n.type).img" class="notif-icon-img" alt="" />
                    <svg v-else-if="notifMeta(n.kind ?? n.type).label === 'cancelled'" viewBox="0 0 24 24" fill="none" stroke="#b91c1c" stroke-width="2" stroke-linecap="round" class="notif-icon-svg">
                      <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
                    </svg>
                    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="notif-icon-svg">
                      <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 01-3.46 0"/>
                    </svg>
                  </div>
                  <div class="notif-body">
                    <div class="notif-title">{{ notifTitle(n) }}</div>
                    <div class="notif-text">{{ notifBody(n) }}</div>
                    <div class="notif-footer">
                      <span class="notif-time">{{ timeAgo(n.created_at) }}</span>
                      <button v-if="n.payload?.retake_id" class="notif-link" @click.stop="goToRetake(n)">Подробнее</button>
                    </div>
                  </div>
                  <div v-if="!n.read_at" class="unread-dot" />
                </li>
              </ul>
            </div>
          </div>

          <!-- Right: calendar + upcoming -->
          <div class="side-col">
            <CalendarWidget :retakes="upcomingRetakes" />
            <UpcomingRetakes :retakes="upcomingRetakes" />
          </div>

        </section>

      </main>
    </div>

  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

.student-root {
  --bg:        #f3f4f7;
  --card:      #ffffff;
  --ink:       #1a1d24;
  --ink-soft:  #6b7280;
  --line:      #d7d9e0;
  --brand:     #3b3fe0;
  --brand-ink: #2a2e9e;
  --radius:    10px;
  --shadow:    0 2px 8px rgba(20,22,60,.07);
  --ease:      cubic-bezier(.2,.7,.2,1);

  min-height: 100dvh;
  font-family: 'Inter', system-ui, sans-serif;
  color: var(--ink);
  -webkit-font-smoothing: antialiased;
}

.page-student { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }

.main { flex: 1; display: flex; flex-direction: column; gap: 20px; padding: 24px; }

/* ── Content columns ── */
.content-cols {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 20px;
  align-items: start;
}

/* ── Feed col (stats + panels + feed) ── */
.feed-col { min-width: 0; display: flex; flex-direction: column; gap: 20px; }

/* ── Stats row ── */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

/* ── Detail panel ── */
.panel-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); overflow: hidden;
  border: 1.5px solid rgba(59,63,224,.15);
}
.panel-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 20px; border-bottom: 1px solid var(--line); gap: 10px;
}
.panel-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 14px; font-weight: 700; color: #3C38B6; margin: 0;
}
.panel-head-actions { display: flex; align-items: center; gap: 8px; }
.panel-close {
  width: 28px; height: 28px; border-radius: 7px; flex-shrink: 0;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s;
}
.panel-close:hover { background: var(--bg); color: var(--ink); }
.panel-close svg { width: 14px; height: 14px; }

/* ── Export buttons ── */
.btn-exp {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px; border-radius: 8px;
  font: 600 12px/1 'Inter', sans-serif; cursor: pointer; white-space: nowrap;
  background: #fff; transition: background .15s, border-color .15s;
}
.btn-exp svg { width: 13px; height: 13px; flex-shrink: 0; }
.btn-exp:disabled { opacity: .4; cursor: not-allowed; }
.btn-exp.excel { border: 1.5px solid #1a7340; color: #1a7340; }
.btn-exp.excel:hover:not(:disabled) { background: rgba(26,115,64,.07); }
.btn-exp.word  { border: 1.5px solid #1a56a0; color: #1a56a0; }
.btn-exp.word:hover:not(:disabled)  { background: rgba(26,86,160,.07); }

.panel-empty {
  padding: 28px 20px; text-align: center;
  font: 13px/1 'Inter', sans-serif; color: var(--ink-soft);
}
.panel-list { list-style: none; margin: 0; padding: 0; }
.panel-row {
  display: flex; flex-direction: column; gap: 4px;
  padding: 11px 20px; border-bottom: 1px solid var(--line);
  transition: background .1s;
}
.panel-row:last-child { border-bottom: none; }
.panel-row:hover { background: rgba(59,63,224,.025); }
.panel-row-main { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.panel-row-meta { display: flex; align-items: center; gap: 12px; }

.panel-disc { font: 500 13px/1.4 'Inter', sans-serif; color: var(--ink); }
.panel-date { font: 11px/1 'Inter', sans-serif; color: var(--ink-soft); }
.panel-room { font: 11px/1 'Inter', sans-serif; color: var(--ink-soft); }
.panel-grade { font: 600 12px/1 'Inter', sans-serif; color: #2e7d32; }

/* ── Discipline rows (панель «Мои дисциплины») ── */
.panel-row--disc {
  flex-direction: row; align-items: center; gap: 12px;
}
.disc-icon {
  width: 34px; height: 34px; border-radius: 9px; flex-shrink: 0;
  background: rgba(59,63,224,.08); display: grid; place-items: center;
  color: var(--brand);
}
.disc-icon svg { width: 16px; height: 16px; }
.disc-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; flex: 1; text-align: left; }
.disc-name { font: 500 13px/1.4 'Inter', sans-serif; color: var(--ink); text-align: left; }
.disc-code { font: 11px/1 'Inter', sans-serif; color: var(--ink-soft); text-align: left; }

.panel-badge {
  flex-shrink: 0; padding: 2px 8px; border-radius: 6px;
  font: 600 11px/1.4 'Inter', sans-serif; white-space: nowrap;
}
.badge-open       { background: #fef3c7; color: #92400e; }
.badge-graded     { background: #d1fae5; color: #065f46; }
.badge-cancelled  { background: #e5e7eb; color: #374151; }

.badge-retake-scheduled   { background: #dbeafe; color: #1e40af; }
.badge-retake-in_progress { background: #fef3c7; color: #92400e; }
.badge-retake-completed   { background: #d1fae5; color: #065f46; }
.badge-retake-cancelled   { background: #e5e7eb; color: #374151; }

/* ── Panel transition ── */
.panel-enter-active { transition: opacity .2s var(--ease), transform .2s var(--ease); }
.panel-leave-active { transition: opacity .15s var(--ease), transform .15s var(--ease); }
.panel-enter-from   { opacity: 0; transform: translateY(-8px); }
.panel-leave-to     { opacity: 0; transform: translateY(-8px); }

/* ── Feed card ── */
.feed-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); overflow: hidden;
}
.feed-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 20px 14px; border-bottom: 1px solid var(--line); gap: 10px;
}
.feed-title-row { display: flex; align-items: center; gap: 8px; }
.feed-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 15px; font-weight: 700; color: #3C38B6; margin: 0;
}
.unread-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 22px; height: 22px; padding: 0 6px; border-radius: 11px;
  background: linear-gradient(135deg, #2b5cff 0%, #8b3df0 100%);
  color: #fff; font: 700 12px/1 'Inter', sans-serif;
}
.mark-all-btn {
  background: none; border: none; cursor: pointer;
  font: 500 12px/1 'Inter', sans-serif; color: var(--brand);
  padding: 4px 0; white-space: nowrap; transition: opacity .15s;
}
.mark-all-btn:hover { opacity: .7; }

.feed-state {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 12px; padding: 48px 20px;
  font: 500 14px/1.5 'Inter', sans-serif; color: var(--ink-soft); text-align: center;
}
.feed-empty svg { width: 40px; height: 40px; opacity: .35; }
.spinner {
  width: 28px; height: 28px; border-radius: 50%;
  border: 3px solid var(--line); border-top-color: var(--brand);
  animation: spin .8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg) } }

.feed-list { list-style: none; margin: 0; padding: 0; }
.feed-item {
  display: flex; align-items: flex-start; gap: 14px;
  padding: 14px 20px; border-bottom: 1px solid var(--line);
  cursor: pointer; transition: background .12s var(--ease);
  position: relative;
}
.feed-item:last-child { border-bottom: none; }
.feed-item:hover { background: rgba(59,63,224,.03); }
.feed-item.unread { background: rgba(59,63,224,.03); }

.notif-icon-wrap {
  width: 42px; height: 42px; border-radius: 12px; flex-shrink: 0;
  display: grid; place-items: center;
}
.notif-icon-img  { width: 26px; height: 26px; object-fit: contain; display: block; }
.notif-icon-svg  { width: 20px; height: 20px; }

.notif-body { flex: 1; min-width: 0; text-align: left; }
.notif-title {
  font: 600 13px/1.4 'Inter', sans-serif; color: var(--ink); text-align: left;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.notif-text {
  font: 13px/1.4 'Inter', sans-serif; color: var(--ink-soft); text-align: left;
  margin-top: 3px;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.notif-footer { display: flex; align-items: center; justify-content: space-between; margin-top: 5px; }
.notif-time { font: 11px/1 'Inter', sans-serif; color: #9ca3af; }
.notif-link {
  background: none; border: none; cursor: pointer; padding: 0;
  font: 500 13px/1 'Inter', sans-serif; color: var(--brand); transition: opacity .15s;
}
.notif-link:hover { opacity: .7; }

.unread-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: var(--brand); flex-shrink: 0; margin-top: 5px;
}

/* ── Side ── */
.side-col { display: flex; flex-direction: column; gap: 16px; }

/* ── Responsive ── */
@media (max-width: 1280px) { .stats-row { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 960px) {
  .content-cols { grid-template-columns: 1fr; }
  .side-col { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
}
@media (max-width: 600px) {
  .main { padding: 16px; gap: 16px; }
  .stats-row { grid-template-columns: 1fr 1fr; }
  .side-col { grid-template-columns: 1fr; }
}
</style>
