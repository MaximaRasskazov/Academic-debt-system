<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'
import { useAuthStore } from '../stores/auth'
import { debtsApi } from '../api/debts'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { directoryApi } from '../api/directory'
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
const myDiscs    = ref([])   // [{id, name, code}]
const debtors    = ref([])   // [{debt_id, student_id, first_name, last_name, middle_name, group_name, discipline_id}]
const discMap    = ref({})   // id → name

const stats = ref([
  { key: 'discs',   label: 'Мои предметы',       value: '—', accent: '#3b3fe0' },
  { key: 'debts',   label: 'Активных долгов',     value: '—', accent: '#e63a5a' },
  { key: 'retakes', label: 'Назначено пересдач',  value: '—', accent: '#f59e0b' },
  { key: 'done',    label: 'Завершено в месяце',  value: '—', accent: '#10b981' },
])

// ── Active panel ──────────────────────────────────────────
const activePanel    = ref(null)   // 'discs' | 'debts' | null
const debtorsLoading = ref(false)
const debtorsByDisc  = ref({})     // discipline_id → [{name, group}]

function togglePanel(key) {
  if (key !== 'discs' && key !== 'debts') return
  if (activePanel.value === key) { activePanel.value = null; return }
  activePanel.value = key
  if (key === 'debts' && debtors.value.length === 0) loadDebtors()
}

async function loadDebtors() {
  debtorsLoading.value = true
  debtorsByDisc.value = {}
  try {
    // Загружаем должников по каждой дисциплине параллельно
    const results = await Promise.allSettled(
      myDiscs.value.map(d => directoryApi.listDebtors(d.id).then(r => ({ discId: d.id, items: r.data.items ?? r.data ?? [] })))
    )
    const map = {}
    for (const r of results) {
      if (r.status !== 'fulfilled') continue
      const { discId, items } = r.value
      if (items.length > 0) {
        map[discId] = items.map(s => ({
          id:    s.student_id,
          name:  [s.last_name, s.first_name, s.middle_name].filter(Boolean).join(' ') || s.student_id,
          group: s.group_name || '',
        }))
      }
    }
    debtorsByDisc.value = map
    debtors.value = Object.values(map).flat()
  } finally {
    debtorsLoading.value = false
  }
}

function exportDebtors(fmt) {
  const headers = ['Дисциплина', 'Студент', 'Группа']
  const rows = []
  for (const disc of myDiscs.value) {
    const list = debtorsByDisc.value[disc.id] ?? []
    for (const s of list) rows.push([disc.name || discMap.value[disc.id] || disc.id, s.name, s.group || '—'])
  }
  const fn = fmt === 'excel' ? exportExcel : exportWord
  fn('Должники по моим предметам', headers, rows, 'dolzhniki')
}

function exportDiscs(fmt) {
  const headers = ['Дисциплина', 'Код']
  const rows = myDiscs.value.map(d => [d.name, d.code || ''])
  const fn = fmt === 'excel' ? exportExcel : exportWord
  fn('Мои предметы', headers, rows, 'predmety')
}

// ── Feed ──────────────────────────────────────────────────
const feed        = ref([])
const feedLoading = ref(true)
const unreadCount = ref(0)

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
      } catch { /* ignore */ }
    }
    ws.onerror = () => {}
  } catch { /* ignore */ }
}

// ── Load ──────────────────────────────────────────────────
onMounted(async () => {
  connectWS()

  const [myDiscRes, allDiscRes, debtsRes, retakesRes, notifsRes] = await Promise.allSettled([
    disciplinesApi.myAsTeacher(),
    disciplinesApi.getAll({ limit: 500 }),
    debtsApi.getByDiscipline(),
    retakesApi.getMy(),
    notificationsApi.getAll({ limit: 30 }),
  ])

  // Карта всех дисциплин id → name
  const allDiscs = allDiscRes.status === 'fulfilled'
    ? (allDiscRes.value.data.items ?? allDiscRes.value.data ?? [])
    : []
  allDiscs.forEach(d => { discMap.value[d.id] = d.name || d.code })

  // Мои дисциплины
  myDiscs.value = myDiscRes.status === 'fulfilled'
    ? (myDiscRes.value.data.items ?? myDiscRes.value.data ?? [])
    : []

  // Активные долги (плоский массив)
  const allDebts = debtsRes.status === 'fulfilled'
    ? (Array.isArray(debtsRes.value.data)
        ? debtsRes.value.data
        : (debtsRes.value.data.items ?? []))
    : []
  const activeDebts = allDebts.filter(d => d.status === 'open').length

  let retakeCount = 0, completedMonth = 0
  if (retakesRes.status === 'fulfilled') {
    const items = retakesRes.value.data.items ?? retakesRes.value.data ?? []
    retakeCount = items.filter(r => r.status === 'scheduled' || r.status === 'in_progress').length
    const now = new Date()
    completedMonth = items.filter(r => {
      if (r.status !== 'completed') return false
      const d = new Date(r.completed_at ?? r.updated_at ?? r.created_at)
      return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth()
    }).length
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
    { key: 'discs',   label: 'Мои предметы',       value: myDiscs.value.length, accent: '#3b3fe0' },
    { key: 'debts',   label: 'Активных долгов',     value: activeDebts,          accent: '#e63a5a' },
    { key: 'retakes', label: 'Назначено пересдач',  value: retakeCount,          accent: '#f59e0b' },
    { key: 'done',    label: 'Завершено в месяце',  value: completedMonth,       accent: '#10b981' },
  ]
})

onUnmounted(() => { ws?.close() })
</script>

<template>
  <div class="teacher-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-teacher">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">

        <!-- ── Content columns ── -->
        <section class="content-cols">

          <!-- Left: stats + panels + feed -->
          <div class="feed-col">

            <!-- Stats -->
            <div class="stats-row">
              <div
                v-for="s in stats" :key="s.key"
                :class="['stat-card', (s.key === 'discs' || s.key === 'debts') && 'stat-card--clickable', activePanel === s.key && 'stat-card--active']"
                @click="togglePanel(s.key)"
              >
                <div class="stat-accent" :style="{ background: s.accent }" />
                <div class="stat-value">{{ s.value }}</div>
                <div class="stat-label">{{ s.label }}</div>
                <div v-if="s.key === 'discs' || s.key === 'debts'" class="stat-hint">
                  {{ activePanel === s.key ? 'Скрыть ↑' : 'Подробнее →' }}
                </div>
              </div>
            </div>

            <!-- Panel: Мои предметы -->
            <Transition name="panel">
              <div v-if="activePanel === 'discs'" class="panel-card">
                <div class="panel-head">
                  <h3 class="panel-title">Мои предметы</h3>
                  <div class="panel-head-actions">
                    <button class="btn-export btn-export--excel" @click="exportDiscs('excel')">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                      Excel
                    </button>
                    <button class="btn-export btn-export--word" @click="exportDiscs('word')">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                      Word
                    </button>
                    <button class="panel-close" @click="activePanel = null">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                    </button>
                  </div>
                </div>
                <div v-if="myDiscs.length === 0" class="panel-empty">Дисциплин не найдено</div>
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

            <!-- Panel: Должники -->
            <Transition name="panel">
              <div v-if="activePanel === 'debts'" class="panel-card">
                <div class="panel-head">
                  <h3 class="panel-title">Должники по моим предметам</h3>
                  <div class="panel-head-actions">
                    <button class="btn-export btn-export--excel" :disabled="debtorsLoading || Object.keys(debtorsByDisc).length === 0" @click="exportDebtors('excel')">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                      Excel
                    </button>
                    <button class="btn-export btn-export--word" :disabled="debtorsLoading || Object.keys(debtorsByDisc).length === 0" @click="exportDebtors('word')">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                      Word
                    </button>
                    <button class="panel-close" @click="activePanel = null">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                    </button>
                  </div>
                </div>

                <div v-if="debtorsLoading" class="panel-loading">
                  <div class="spinner-sm" /><span>Загрузка…</span>
                </div>

                <div v-else-if="Object.keys(debtorsByDisc).length === 0" class="panel-empty">
                  Должников нет
                </div>

                <div v-else class="debtors-body">
                  <div v-for="disc in myDiscs.filter(d => debtorsByDisc[d.id])" :key="disc.id" class="debtors-group">
                    <div class="debtors-group-head">
                      <span class="debtors-disc-name">{{ disc.name || discMap[disc.id] || disc.id }}</span>
                      <span class="debtors-count">{{ debtorsByDisc[disc.id].length }}</span>
                    </div>
                    <ul class="panel-list">
                      <li v-for="s in debtorsByDisc[disc.id]" :key="s.id" class="panel-row panel-row--student">
                        <div class="student-avatar">{{ s.name.charAt(0) }}</div>
                        <div class="student-body">
                          <span class="student-name">{{ s.name }}</span>
                          <span v-if="s.group" class="student-group">{{ s.group }}</span>
                        </div>
                      </li>
                    </ul>
                  </div>
                </div>
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

.teacher-root {
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

.page-teacher { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }

.main { flex: 1; display: flex; flex-direction: column; gap: 20px; padding: 24px; }

/* ── Content columns ── */
.content-cols {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 20px;
  align-items: start;
}

/* ── Feed col ── */
.feed-col { min-width: 0; display: flex; flex-direction: column; gap: 20px; }

/* ── Stats row ── */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.stat-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 20px 20px 16px 24px;
  position: relative; overflow: hidden;
  transition: box-shadow .18s var(--ease), transform .18s var(--ease);
}
.stat-card--clickable { cursor: pointer; }
.stat-card--clickable:hover {
  box-shadow: 0 4px 18px rgba(20,22,60,.13);
  transform: translateY(-2px);
}
.stat-card--active {
  box-shadow: 0 0 0 2px var(--brand), 0 4px 18px rgba(59,63,224,.15);
  transform: translateY(-2px);
}
.stat-accent { position: absolute; left: 0; top: 0; bottom: 0; width: 4px; }
.stat-value  { font-size: 34px; font-weight: 700; line-height: 1; margin-bottom: 8px; }
.stat-label  { font-size: 12px; color: var(--ink-soft); line-height: 1.4; }
.stat-hint   { font-size: 11px; color: var(--brand); margin-top: 6px; font-weight: 500; }

/* ── Panel card ── */
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
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
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
.btn-export {
  display: inline-flex; align-items: center; gap: 5px;
  height: 28px; padding: 0 10px; border-radius: 7px; border: 1.5px solid var(--line);
  font: 600 11px/1 'Inter', sans-serif; cursor: pointer; white-space: nowrap;
  transition: border-color .15s, background .15s, color .15s;
}
.btn-export svg { width: 12px; height: 12px; flex-shrink: 0; }
.btn-export:disabled { opacity: .4; cursor: not-allowed; }
.btn-export--excel { background: #f0fdf4; color: #166534; border-color: #bbf7d0; }
.btn-export--excel:hover:not(:disabled) { background: #dcfce7; border-color: #86efac; }
.btn-export--word  { background: #eff6ff; color: #1e40af; border-color: #bfdbfe; }
.btn-export--word:hover:not(:disabled)  { background: #dbeafe; border-color: #93c5fd; }

.panel-empty {
  padding: 28px 20px; text-align: center;
  font: 13px/1 'Inter', sans-serif; color: var(--ink-soft);
}
.panel-loading {
  display: flex; align-items: center; gap: 10px;
  padding: 20px; font: 13px/1 'Inter', sans-serif; color: var(--ink-soft);
}
.spinner-sm {
  width: 18px; height: 18px; border-radius: 50%;
  border: 2.5px solid var(--line); border-top-color: var(--brand);
  animation: spin .8s linear infinite; flex-shrink: 0;
}

.panel-list { list-style: none; margin: 0; padding: 0; }

/* Дисциплины */
.panel-row--disc {
  display: flex; align-items: center; gap: 12px;
  padding: 12px 20px; border-bottom: 1px solid var(--line);
  transition: background .1s;
}
.panel-row--disc:last-child { border-bottom: none; }
.panel-row--disc:hover { background: rgba(59,63,224,.025); }
.disc-icon {
  width: 34px; height: 34px; border-radius: 9px; flex-shrink: 0;
  background: rgba(59,63,224,.08); display: grid; place-items: center;
  color: var(--brand);
}
.disc-icon svg { width: 16px; height: 16px; }
.disc-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; text-align: left; }
.disc-name { font: 500 13px/1.4 'Inter', sans-serif; color: var(--ink); text-align: left; }
.disc-code { font: 11px/1 'Inter', sans-serif; color: var(--ink-soft); text-align: left; }

/* Должники — секции по дисциплинам */
.debtors-body { overflow: hidden; }
.debtors-group { border-bottom: 1px solid var(--line); }
.debtors-group:last-child { border-bottom: none; }
.debtors-group-head {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  padding: 10px 20px 8px;
  background: rgba(59,63,224,.03);
  border-bottom: 1px solid var(--line);
}
.debtors-disc-name { font: 600 12px/1.4 'Inter', sans-serif; color: var(--brand-ink); }
.debtors-count {
  font: 700 11px/1 'Inter', sans-serif;
  background: rgba(230,60,90,.12); color: #b91c1c;
  padding: 2px 7px; border-radius: 10px;
}

/* Студенты */
.panel-row--student {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 20px; border-bottom: 1px solid var(--line);
  transition: background .1s;
}
.panel-row--student:last-child { border-bottom: none; }
.panel-row--student:hover { background: rgba(59,63,224,.025); }
.student-avatar {
  width: 30px; height: 30px; border-radius: 50%; flex-shrink: 0;
  background: linear-gradient(135deg, #3b3fe0, #8b3df0);
  display: grid; place-items: center;
  font: 700 13px/1 'Inter', sans-serif; color: #fff;
}
.student-body { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.student-name  { font: 500 13px/1.4 'Inter', sans-serif; color: var(--ink); }
.student-group { font: 11px/1 'Inter', sans-serif; color: var(--ink-soft); }

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
.notif-icon-img { width: 26px; height: 26px; object-fit: contain; display: block; }
.notif-icon-svg { width: 20px; height: 20px; }

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

.side-col { display: flex; flex-direction: column; gap: 16px; }

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
