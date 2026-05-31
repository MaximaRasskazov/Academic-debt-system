<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'
import { useAuthStore } from '../stores/auth'
import { debtsApi } from '../api/debts'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { notificationsApi } from '../api/notifications'
import { partsInTZ } from '../utils/datetime'

const auth = useAuthStore()
const sidebarOpen = ref(false)

// ── Stats ─────────────────────────────────────────────────
const upcomingRetakes = ref([])
const stats = ref([
  { label: 'Моих долгов',        value: '—', accent: '#e63c5a' },
  { label: 'Дисциплин',          value: '—', accent: '#3b3fe0' },
  { label: 'Назначено пересдач', value: '—', accent: '#f59e0b' },
  { label: 'Завершено',          value: '—', accent: '#10b981' },
])

// ── Feed ──────────────────────────────────────────────────
const feed         = ref([])
const feedLoading  = ref(true)
const unreadCount  = ref(0)

const NOTIF_ICONS = {
  retake_assigned:  'calendar',
  retake_updated:   'edit',
  retake_cancelled: 'x-circle',
  grade_set:        'award',
  default:          'bell',
}

function notifIcon(type) { return NOTIF_ICONS[type] || NOTIF_ICONS.default }

function timeAgo(iso) {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60000)
  if (m < 1)  return 'только что'
  if (m < 60) return `${m} мин назад`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} ч назад`
  const d = Math.floor(h / 24)
  if (d < 7)  return `${d} д назад`
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: 'short' })
}

async function markRead(n) {
  if (n.is_read) return
  n.is_read = true
  unreadCount.value = Math.max(0, unreadCount.value - 1)
  try { await notificationsApi.markRead(n.id) } catch { /* silent */ }
}

async function markAllRead() {
  const unread = feed.value.filter(n => !n.is_read)
  unread.forEach(n => { n.is_read = true })
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
        if (!n.is_read) unreadCount.value++
      } catch { /* ignore non-json */ }
    }
    ws.onerror = () => {}
  } catch { /* ignore ws errors */ }
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

  const discMap = {}
  const myDiscs = discRes.status === 'fulfilled'
    ? (discRes.value.data.items ?? discRes.value.data ?? [])
    : []
  myDiscs.forEach(d => { discMap[d.id] = d.name || d.code })

  const myDebts = debtsRes.status === 'fulfilled'
    ? (debtsRes.value.data.items ?? debtsRes.value.data ?? [])
    : []
  const openDebts = myDebts.filter(d => d.status === 'open').length

  let scheduledCount = 0, completedCount = 0
  if (retakesRes.status === 'fulfilled') {
    const items = retakesRes.value.data.items ?? retakesRes.value.data ?? []
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
          subject: discMap[r.discipline_id] || 'Дисциплина',
          day: Number(p.day), month: Number(p.month), year: Number(p.year),
          time: `${p.hour}:${p.minute}`,
          building: r.building, room: r.room,
        }
      })
  }

  if (notifsRes.status === 'fulfilled') {
    feed.value = notifsRes.value.data.items ?? notifsRes.value.data ?? []
    unreadCount.value = feed.value.filter(n => !n.is_read).length
  }
  feedLoading.value = false

  stats.value = [
    { label: 'Моих долгов',        value: openDebts,       accent: '#e63c5a' },
    { label: 'Дисциплин',          value: myDiscs.length,  accent: '#3b3fe0' },
    { label: 'Назначено пересдач', value: scheduledCount,  accent: '#f59e0b' },
    { label: 'Завершено',          value: completedCount,  accent: '#10b981' },
  ]
})

onUnmounted(() => { ws?.close() })
</script>

<template>
  <div class="student-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-student">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">

        <!-- ── Stats row ── -->
        <section class="stats-row">
          <div v-for="s in stats" :key="s.label" class="stat-card">
            <div class="stat-accent" :style="{ background: s.accent }" />
            <div class="stat-value">{{ s.value }}</div>
            <div class="stat-label">{{ s.label }}</div>
          </div>
        </section>

        <!-- ── Content columns ── -->
        <section class="content-cols">

          <!-- News feed -->
          <div class="feed-col">
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
                  :class="{ unread: !n.is_read }"
                  @click="markRead(n)"
                >
                  <div class="notif-icon-wrap" :class="'icon-' + notifIcon(n.type)">
                    <!-- calendar -->
                    <svg v-if="notifIcon(n.type) === 'calendar'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                      <rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/>
                    </svg>
                    <!-- edit -->
                    <svg v-else-if="notifIcon(n.type) === 'edit'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                      <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                      <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                    </svg>
                    <!-- x-circle -->
                    <svg v-else-if="notifIcon(n.type) === 'x-circle'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                      <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
                    </svg>
                    <!-- award / grade -->
                    <svg v-else-if="notifIcon(n.type) === 'award'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                      <circle cx="12" cy="8" r="6"/><path d="M15.477 12.89L17 22l-5-3-5 3 1.523-9.11"/>
                    </svg>
                    <!-- bell default -->
                    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                      <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 01-3.46 0"/>
                    </svg>
                  </div>
                  <div class="notif-body">
                    <div class="notif-title">{{ n.title || 'Уведомление' }}</div>
                    <div v-if="n.body" class="notif-text">{{ n.body }}</div>
                    <div class="notif-time">{{ timeAgo(n.created_at) }}</div>
                  </div>
                  <div v-if="!n.is_read" class="unread-dot" />
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

/* ── Stats row ── */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.stat-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 20px 20px 20px 24px;
  position: relative; overflow: hidden;
}
.stat-accent { position: absolute; left: 0; top: 0; bottom: 0; width: 4px; }
.stat-value  { font-size: 34px; font-weight: 700; line-height: 1; margin-bottom: 8px; }
.stat-label  { font-size: 12px; color: var(--ink-soft); line-height: 1.4; }

/* ── Content columns ── */
.content-cols {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 20px;
  align-items: start;
}

/* ── Feed ── */
.feed-col { min-width: 0; }
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
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
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
  padding: 4px 0; white-space: nowrap;
  transition: opacity .15s;
}
.mark-all-btn:hover { opacity: .7; }

/* ── Feed states ── */
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

/* ── Feed list ── */
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

/* ── Notif icon ── */
.notif-icon-wrap {
  width: 38px; height: 38px; border-radius: 10px; flex-shrink: 0;
  display: grid; place-items: center;
}
.notif-icon-wrap svg { width: 18px; height: 18px; }

.icon-calendar   { background: rgba(59,63,224,.1);   color: var(--brand); }
.icon-edit       { background: rgba(245,158,11,.1);  color: #b45309; }
.icon-x-circle   { background: rgba(220,38,38,.08);  color: #b91c1c; }
.icon-award      { background: rgba(16,185,129,.1);  color: #065f46; }
.icon-bell       { background: rgba(107,114,128,.1); color: var(--ink-soft); }

/* ── Notif body ── */
.notif-body { flex: 1; min-width: 0; }
.notif-title {
  font: 600 13px/1.4 'Inter', sans-serif; color: var(--ink);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.notif-text {
  font: 13px/1.4 'Inter', sans-serif; color: var(--ink-soft);
  margin-top: 3px;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.notif-time { font: 11px/1 'Inter', sans-serif; color: #9ca3af; margin-top: 5px; }

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
