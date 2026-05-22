<script setup>
import { ref, computed, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const sidebarOpen = ref(false)

function logout() {
  auth.logout()
  router.push('/login')
}

// --- Calendar ---
const today = new Date()
const calendarCursor = ref(new Date(today.getFullYear(), today.getMonth(), 1))

const WEEK_DAYS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

const calendarTitle = computed(() =>
  calendarCursor.value.toLocaleDateString('ru-RU', { month: 'long', year: 'numeric' })
)

const calendarCells = computed(() => {
  const year = calendarCursor.value.getFullYear()
  const month = calendarCursor.value.getMonth()
  const firstDow = new Date(year, month, 1).getDay()
  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const leading = firstDow === 0 ? 6 : firstDow - 1
  const cells = Array(leading).fill(null)
  for (let d = 1; d <= daysInMonth; d++) cells.push(d)
  return cells
})

function prevMonth() {
  const d = calendarCursor.value
  calendarCursor.value = new Date(d.getFullYear(), d.getMonth() - 1, 1)
}
function nextMonth() {
  const d = calendarCursor.value
  calendarCursor.value = new Date(d.getFullYear(), d.getMonth() + 1, 1)
}
function isToday(day) {
  return day !== null &&
    day === today.getDate() &&
    calendarCursor.value.getMonth() === today.getMonth() &&
    calendarCursor.value.getFullYear() === today.getFullYear()
}

// --- Mock данные: ближайшие пересдачи (заменить на API) ---
const upcomingRetakes = [
  { id: 1, subject: 'Математический анализ', day: 25, month: 4, year: 2026, time: '10:00', building: '1', room: '204' },
  { id: 2, subject: 'Физика',                day: 27, month: 4, year: 2026, time: '14:00', building: '2', room: '308' },
  { id: 3, subject: 'Линейная алгебра',      day: 30, month: 4, year: 2026, time: '09:00', building: '3', room: '112' },
]

function getRetakesForDay(day) {
  if (!day) return []
  const y = calendarCursor.value.getFullYear()
  const m = calendarCursor.value.getMonth()
  return upcomingRetakes.filter(r => r.day === day && r.month === m && r.year === y)
}

function hasRetake(day) {
  return getRetakesForDay(day).length > 0
}

function formatDayLabel(r) {
  return new Date(r.year, r.month, r.day)
    .toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
}

// --- Модальное окно пересдач на дату ---
const modal = reactive({ open: false, day: null, retakes: [] })

const modalDateLabel = computed(() => {
  if (!modal.day) return ''
  const y = calendarCursor.value.getFullYear()
  const m = calendarCursor.value.getMonth()
  return new Date(y, m, modal.day).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
})

function onDayClick(day) {
  if (!day) return
  const retakes = getRetakesForDay(day)
  if (!retakes.length) return
  modal.day = day
  modal.retakes = retakes
  modal.open = true
}

// --- Dashboard stats ---
const stats = [
  { label: 'Академических долгов', value: 156, accent: '#e63c5a' },
  { label: 'Назначено пересдач',   value: 12,  accent: '#3b3fe0' },
  { label: 'Проводится сейчас',    value: 2,   accent: '#f59e0b' },
  { label: 'Завершено в месяце',   value: 34,  accent: '#10b981' },
]

// --- Форма назначения пересдачи ---
const retakeForm = reactive({
  subject:  '',
  type:     'normal',
  date:     '',
  time:     '',
  duration: 90,
  building: '',
  room:     '',
})

function submitRetake() {
  // TODO: POST /api/retakes
  console.log('create retake', { ...retakeForm })
}
</script>

<template>
  <div class="dean-root">

    <!-- Overlay сайдбара -->
    <div class="sidebar-overlay" :class="{ active: sidebarOpen }" @click="sidebarOpen = false" />

    <!-- Левое меню -->
    <aside class="sidebar" :class="{ open: sidebarOpen }">
      <div class="sidebar-brand">Академический<br>Ассистент</div>
      <nav class="sidebar-nav">
        <div class="nav-section">
          <div class="nav-section-title">Центр заявок</div>
          <button class="nav-item nav-item--sub">Пересдачи</button>
          <button class="nav-item nav-item--sub">Изменения времени</button>
          <button class="nav-item nav-item--sub">Смена роли</button>
        </div>
        <div class="nav-section">
          <button class="nav-item">Пользователи</button>
        </div>
        <div class="nav-section">
          <button class="nav-item">Ведомость</button>
        </div>
      </nav>
      <div class="sidebar-footer">
        <button class="logout-btn" @click="logout">Выйти</button>
      </div>
    </aside>

    <!-- Модальное окно: пересдачи на выбранный день -->
    <Transition name="modal">
      <div v-if="modal.open" class="modal-overlay" @click.self="modal.open = false">
        <div class="modal-card">
          <div class="modal-head">
            <h3 class="modal-title">Пересдачи · {{ modalDateLabel }}</h3>
            <button class="modal-close" @click="modal.open = false" aria-label="Закрыть">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                <path d="M18 6L6 18M6 6l12 12"/>
              </svg>
            </button>
          </div>
          <ul class="modal-list">
            <li v-for="r in modal.retakes" :key="r.id" class="modal-item">
              <div class="notif-icon notif-icon--retake">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M9 12h6M9 16h6M7 4h10a2 2 0 012 2v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z"/>
                </svg>
              </div>
              <div class="notif-body">
                <div class="notif-subject">{{ r.subject }}</div>
                <div class="notif-tags">
                  <span class="notif-tag">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="3" y1="10" x2="21" y2="10"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="16" y1="2" x2="16" y2="6"/></svg>
                    {{ formatDayLabel(r) }}
                  </span>
                  <span class="notif-tag">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/></svg>
                    {{ r.time }}
                  </span>
                  <span class="notif-tag">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7z"/><circle cx="12" cy="9" r="2.5"/></svg>
                    корп. {{ r.building }}, ауд. {{ r.room }}
                  </span>
                </div>
              </div>
            </li>
          </ul>
        </div>
      </div>
    </Transition>

    <!-- Страница -->
    <div class="page-dean">

      <!-- Шапка -->
      <header class="header">
        <div class="header-left">
          <button class="burger" @click="sidebarOpen = true" aria-label="Открыть меню">
            <span /><span /><span />
          </button>
          <span class="app-name">Академический Ассистент</span>
        </div>
        <div class="header-right">
          <div class="user-meta">
            <span class="user-name">{{ auth.user?.lastName }} {{ auth.user?.firstName }} {{ auth.user?.middleName }}</span>
            <span class="user-role">Деканат</span>
          </div>
          <RouterLink to="/profile" class="avatar">
            <img v-if="auth.user?.avatar" :src="auth.user.avatar" alt="Фото профиля" />
            <span v-else>{{ auth.user?.firstName?.[0] }}{{ auth.user?.lastName?.[0] }}</span>
          </RouterLink>
        </div>
      </header>

      <!-- Основной контент -->
      <main class="main">

        <!-- Левая колонка 70% -->
        <section class="col-left">

          <div class="stats-grid">
            <div v-for="s in stats" :key="s.label" class="stat-card">
              <div class="stat-accent" :style="{ background: s.accent }" />
              <div class="stat-value">{{ s.value }}</div>
              <div class="stat-label">{{ s.label }}</div>
            </div>
          </div>

          <div class="section-card">
            <h2 class="section-title">Назначить пересдачу</h2>
            <form class="retake-form" @submit.prevent="submitRetake" novalidate>

              <div class="form-row">
                <div class="field">
                  <label>Дисциплина</label>
                  <input class="input" v-model="retakeForm.subject" placeholder="Название дисциплины" />
                </div>
                <div class="field">
                  <label>Тип пересдачи</label>
                  <select class="input" v-model="retakeForm.type">
                    <option value="normal">Обычная</option>
                    <option value="commission">С комиссией (мин. 3 преподавателя)</option>
                  </select>
                </div>
              </div>

              <div class="form-row">
                <div class="field">
                  <label>Дата</label>
                  <div class="date-wrap">
                    <input class="input" type="date" v-model="retakeForm.date" />
                    <svg class="date-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="3" y="4" width="18" height="18" rx="2"/><line x1="3" y1="10" x2="21" y2="10"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="16" y1="2" x2="16" y2="6"/>
                    </svg>
                  </div>
                </div>
                <div class="field">
                  <label>Время</label>
                  <input class="input" type="time" v-model="retakeForm.time" />
                </div>
                <div class="field">
                  <label>Длительность (мин)</label>
                  <input class="input" type="number" v-model="retakeForm.duration" min="15" max="480" />
                </div>
              </div>

              <div class="form-row">
                <div class="field">
                  <label>Корпус</label>
                  <input class="input" v-model="retakeForm.building" placeholder="№ корпуса" />
                </div>
                <div class="field">
                  <label>Аудитория</label>
                  <input class="input" v-model="retakeForm.room" placeholder="№ аудитории" />
                </div>
              </div>

              <button class="btn-primary" type="submit">Назначить пересдачу</button>
            </form>
          </div>

        </section>

        <!-- Правая колонка 30% -->
        <section class="col-right">

          <!-- Календарь -->
          <div class="section-card">
            <div class="calendar-header">
              <button class="cal-nav" @click="prevMonth">&#8249;</button>
              <span class="cal-title">{{ calendarTitle }}</span>
              <button class="cal-nav" @click="nextMonth">&#8250;</button>
            </div>
            <div class="calendar-grid">
              <div v-for="d in WEEK_DAYS" :key="d" class="cal-weekday">{{ d }}</div>
              <div
                v-for="(day, i) in calendarCells"
                :key="i"
                class="cal-cell"
                :class="{ empty: day === null, clickable: hasRetake(day) }"
                @click="onDayClick(day)"
              >
                <span class="cal-num" :class="{ today: isToday(day) }">{{ day }}</span>
                <span v-if="hasRetake(day)" class="cal-dot" />
              </div>
            </div>
          </div>

          <!-- Ближайшие пересдачи -->
          <div class="section-card">
            <h2 class="section-title">Ближайшие пересдачи</h2>
            <ul class="notif-list">
              <li v-for="r in upcomingRetakes" :key="r.id" class="notif-item">
                <div class="notif-icon notif-icon--retake">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M9 12h6M9 16h6M7 4h10a2 2 0 012 2v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z"/>
                  </svg>
                </div>
                <div class="notif-body">
                  <div class="notif-subject">{{ r.subject }}</div>
                  <div class="notif-tags">
                    <span class="notif-tag">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="3" y1="10" x2="21" y2="10"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="16" y1="2" x2="16" y2="6"/></svg>
                      {{ formatDayLabel(r) }}
                    </span>
                    <span class="notif-tag">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/></svg>
                      {{ r.time }}
                    </span>
                    <span class="notif-tag">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7z"/><circle cx="12" cy="9" r="2.5"/></svg>
                      корп. {{ r.building }}, ауд. {{ r.room }}
                    </span>
                  </div>
                </div>
              </li>
            </ul>
          </div>

        </section>

      </main>
    </div>

  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

.dean-root {
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

.page-dean {
  min-height: 100dvh;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

/* ── Header ── */
.header {
  height: 64px;
  background: var(--card);
  border-bottom: 1px solid var(--line);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 10;
  flex-shrink: 0;
}
.header-left { display: flex; align-items: center; gap: 16px; }
.burger {
  background: none; border: none; cursor: pointer; padding: 6px;
  display: flex; flex-direction: column; justify-content: center; gap: 5px;
  border-radius: 6px; transition: background .15s var(--ease);
}
.burger:hover { background: var(--bg); }
.burger span { display: block; width: 22px; height: 2px; background: var(--ink); border-radius: 2px; }
.app-name { font-size: 16px; font-weight: 600; color: var(--brand-ink); }
.header-right { display: flex; align-items: center; gap: 14px; }
.user-meta { display: flex; flex-direction: column; align-items: flex-end; gap: 2px; }
.user-name { font-size: 14px; font-weight: 500; white-space: nowrap; }
.user-role { font-size: 12px; color: var(--ink-soft); }
.avatar {
  width: 40px; height: 40px; border-radius: 50%; flex-shrink: 0;
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  display: grid; place-items: center;
  color: #fff; font-weight: 700; font-size: 14px;
  text-decoration: none; overflow: hidden; cursor: pointer;
}
.avatar img { width: 100%; height: 100%; object-fit: cover; }

/* ── Main grid ── */
.main {
  flex: 1;
  display: grid;
  grid-template-columns: 7fr 3fr;
  gap: 24px;
  padding: 24px;
  align-items: start;
}

/* ── Stats ── */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}
.stat-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 20px 20px 20px 24px;
  position: relative; overflow: hidden;
}
.stat-accent { position: absolute; left: 0; top: 0; bottom: 0; width: 4px; }
.stat-value { font-size: 34px; font-weight: 700; line-height: 1; margin-bottom: 8px; }
.stat-label { font-size: 12px; color: var(--ink-soft); line-height: 1.4; }

/* ── Section card ── */
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px; margin-bottom: 20px;
}
.section-card:last-child { margin-bottom: 0; }
.section-title { font-size: 15px; font-weight: 600; margin: 0 0 20px; }

/* ── Retake form ── */
.retake-form { display: flex; flex-direction: column; gap: 16px; }
.form-row { display: flex; gap: 16px; }
.field { flex: 1; display: flex; flex-direction: column; gap: 6px; }
.field label { font-size: 13px; font-weight: 500; color: var(--ink); text-align: left; }
.input {
  appearance: none; height: 38px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.input::placeholder { color: #b7b9c2; }

/* Date input с иконкой-календарём */
.date-wrap { position: relative; }
.date-wrap .input { padding-right: 36px; cursor: pointer; }
.date-ico {
  position: absolute; right: 10px; top: 50%; transform: translateY(-50%);
  width: 16px; height: 16px; color: var(--ink-soft); pointer-events: none;
}
.date-wrap .input::-webkit-calendar-picker-indicator {
  position: absolute; right: 0; top: 0;
  width: 100%; height: 100%; opacity: 0; cursor: pointer;
}

.btn-primary {
  align-self: center;
  padding: 0 32px; height: 40px; border: none;
  border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 14px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 8px 20px -8px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-primary:hover { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }

/* ── Calendar ── */
.calendar-header {
  display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px;
}
.cal-title { font-size: 14px; font-weight: 600; text-transform: capitalize; }
.cal-nav {
  background: none; border: none; cursor: pointer;
  font-size: 22px; color: var(--ink-soft);
  padding: 2px 8px; border-radius: 6px; line-height: 1;
  transition: background .15s, color .15s;
}
.cal-nav:hover { background: var(--bg); color: var(--ink); }

.calendar-grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: 2px; }
.cal-weekday {
  font-size: 11px; font-weight: 600; color: var(--ink-soft);
  text-align: center; padding-bottom: 8px;
}

.cal-cell {
  display: flex; flex-direction: column; align-items: center;
  gap: 3px; padding: 3px 0; border-radius: 6px;
  transition: background .15s;
}
.cal-cell.clickable { cursor: pointer; }
.cal-cell.clickable:hover { background: rgba(245,158,11,.08); }
.cal-cell.empty { pointer-events: none; }

.cal-num {
  width: 26px; height: 26px;
  display: grid; place-items: center;
  font-size: 13px; border-radius: 50%;
  transition: background .15s, color .15s;
}
.cal-cell:not(.empty):not(.clickable):hover .cal-num { background: var(--bg); }
.cal-num.today {
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  color: #fff; font-weight: 700;
}

.cal-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: #f59e0b;
  flex-shrink: 0;
}

/* ── Notifications / upcoming retakes ── */
.notif-list { list-style: none; padding: 0; margin: 0; }
.notif-item {
  display: flex; align-items: flex-start; gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--line);
}
.notif-item:last-child { border-bottom: none; padding-bottom: 0; }
.notif-item:first-child { padding-top: 0; }

.notif-icon {
  width: 38px; height: 38px; border-radius: 50%;
  display: grid; place-items: center; flex-shrink: 0;
}
.notif-icon--retake { background: rgba(59,63,224,.1); color: var(--brand); }
.notif-icon svg { width: 18px; height: 18px; }

.notif-body { flex: 1; min-width: 0; }
.notif-subject {
  font-size: 13px; font-weight: 600; color: var(--ink);
  margin-bottom: 6px; line-height: 1.3;
}
.notif-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.notif-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 3px 8px;
  background: var(--bg); border-radius: 20px;
  font-size: 11px; color: var(--ink-soft);
}
.notif-tag svg { width: 11px; height: 11px; flex-shrink: 0; }

/* ── Modal ── */
.modal-overlay {
  position: fixed; inset: 0;
  background: rgba(10,12,30,.45);
  z-index: 200;
  display: grid; place-items: center; padding: 24px;
}
.modal-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: 0 24px 48px -12px rgba(20,22,60,.25);
  width: min(440px, 100%); max-height: 80vh;
  overflow-y: auto; padding: 24px;
}
.modal-head {
  display: flex; align-items: center;
  justify-content: space-between; margin-bottom: 20px;
}
.modal-title { font-size: 15px; font-weight: 600; margin: 0; }
.modal-close {
  background: none; border: none; cursor: pointer; padding: 4px;
  color: var(--ink-soft); border-radius: 6px; display: grid; place-items: center;
  transition: background .15s, color .15s;
}
.modal-close:hover { background: var(--bg); color: var(--ink); }
.modal-close svg { width: 18px; height: 18px; }
.modal-list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 12px; }
.modal-item { display: flex; align-items: flex-start; gap: 12px; }

/* ── Modal transition ── */
.modal-enter-active { transition: opacity .2s cubic-bezier(.2,.7,.2,1); }
.modal-leave-active { transition: opacity .15s cubic-bezier(.2,.7,.2,1); }
.modal-enter-from, .modal-leave-to { opacity: 0; }

/* ── Sidebar ── */
.sidebar-overlay {
  position: fixed; inset: 0;
  background: rgba(10,12,30,.45); z-index: 99;
  opacity: 0; pointer-events: none;
  transition: opacity .25s cubic-bezier(.2,.7,.2,1);
}
.sidebar-overlay.active { opacity: 1; pointer-events: all; }

.sidebar {
  position: fixed; top: 0; left: 0; bottom: 0; width: 280px;
  background: var(--card); box-shadow: 4px 0 24px rgba(20,22,60,.12);
  z-index: 100; display: flex; flex-direction: column;
  transform: translateX(-100%);
  transition: transform .3s cubic-bezier(.2,.7,.2,1);
}
.sidebar.open { transform: translateX(0); }
.sidebar-brand {
  padding: 28px 24px; text-align: center;
  font-size: 15px; font-weight: 700; text-transform: uppercase;
  letter-spacing: .06em; color: var(--brand-ink);
  border-bottom: 1px solid var(--line); line-height: 1.5;
}
.sidebar-nav { flex: 1; overflow-y: auto; padding: 12px 0; }
.nav-section { margin-bottom: 4px; }
.nav-section-title {
  padding: 12px 20px 6px; font-size: 11px; font-weight: 700;
  text-transform: uppercase; letter-spacing: .08em; color: var(--ink-soft);
}
.nav-item {
  display: block; width: 100%; text-align: left;
  background: none; border: none; padding: 10px 20px;
  font: 500 14px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer;
  transition: background .15s, color .15s;
}
.nav-item:hover { background: rgba(59,63,224,.07); color: var(--brand-ink); }
.nav-item--sub { padding-left: 32px; font-weight: 400; font-size: 13px; color: var(--ink-soft); }
.nav-item--sub:hover { color: var(--brand-ink); }
.sidebar-footer { padding: 16px 20px; border-top: 1px solid var(--line); }
.logout-btn {
  width: 100%; height: 40px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: transparent; color: #e63c5a;
  font: 500 14px/1 'Inter', sans-serif; cursor: pointer;
  transition: background .15s, border-color .15s;
}
.logout-btn:hover { background: #fff0f2; border-color: #e63c5a; }

/* ── Responsive ── */
@media (max-width: 1280px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 960px) {
  .main { grid-template-columns: 1fr; }
  .col-right { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
  .col-right .section-card { margin-bottom: 0; }
}
@media (max-width: 600px) {
  .main { padding: 16px; gap: 16px; }
  .form-row { flex-direction: column; gap: 12px; }
  .stats-grid { grid-template-columns: 1fr 1fr; }
  .col-right { grid-template-columns: 1fr; }
}
</style>
