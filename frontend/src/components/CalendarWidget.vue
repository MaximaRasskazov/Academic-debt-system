<script setup>
import { ref, computed, reactive } from 'vue'

const props = defineProps({
  retakes: { type: Array, default: () => [] },
})

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

function getRetakesForDay(day) {
  if (!day) return []
  const y = calendarCursor.value.getFullYear()
  const m = calendarCursor.value.getMonth()
  return props.retakes.filter(r => r.day === day && r.month === m && r.year === y)
}

function hasRetake(day) {
  return getRetakesForDay(day).length > 0
}

function formatDayLabel(r) {
  return new Date(r.year, r.month, r.day)
    .toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
}

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
</script>

<template>
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

  <!-- Модальное окно -->
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
            <div class="notif-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M9 12h6M9 16h6M7 4h10a2 2 0 012 2v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z"/>
              </svg>
            </div>
            <div class="notif-body">
              <div class="notif-subject">{{ r.subject }}</div>
              <div class="notif-details">
                <div class="notif-detail-row">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="3" y1="10" x2="21" y2="10"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="16" y1="2" x2="16" y2="6"/></svg>
                  {{ formatDayLabel(r) }}
                </div>
                <div class="notif-detail-row">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/></svg>
                  {{ r.time }}
                </div>
                <div class="notif-detail-row">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7z"/><circle cx="12" cy="9" r="2.5"/></svg>
                  корп. {{ r.building }}, ауд. {{ r.room }}
                </div>
              </div>
            </div>
          </li>
        </ul>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px; margin-bottom: 20px;
}

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
  gap: 3px; padding: 3px 0; border-radius: 6px; transition: background .15s;
}
.cal-cell.clickable { cursor: pointer; }
.cal-cell.clickable:hover { background: rgba(245,158,11,.08); }
.cal-cell.empty { pointer-events: none; }
.cal-num {
  width: 26px; height: 26px; display: grid; place-items: center;
  font-size: 13px; border-radius: 50%; transition: background .15s, color .15s;
}
.cal-cell:not(.empty):not(.clickable):hover .cal-num { background: var(--bg); }
.cal-num.today {
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  color: #fff; font-weight: 700;
}
.cal-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: #f59e0b; flex-shrink: 0;
}

/* Modal */
.modal-overlay {
  position: fixed; inset: 0;
  background: rgba(10,12,30,.45); z-index: 200;
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
.modal-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0;
}
.modal-close {
  background: none; border: none; cursor: pointer; padding: 4px;
  color: var(--ink-soft); border-radius: 6px; display: grid; place-items: center;
  transition: background .15s, color .15s;
}
.modal-close:hover { background: var(--bg); color: var(--ink); }
.modal-close svg { width: 18px; height: 18px; }
.modal-list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 12px; }
.modal-item { display: flex; align-items: flex-start; gap: 12px; }

.notif-icon {
  width: 38px; height: 38px; border-radius: 50%;
  display: grid; place-items: center; flex-shrink: 0;
  background: rgba(59,63,224,.1); color: var(--brand);
}
.notif-icon svg { width: 18px; height: 18px; }
.notif-body { flex: 1; min-width: 0; }
.notif-subject {
  font-size: 13px; font-weight: 600; color: var(--ink);
  margin-bottom: 8px; line-height: 1.3; text-align: left;
}
.notif-details { display: flex; flex-direction: column; gap: 5px; }
.notif-detail-row {
  display: flex; align-items: center; gap: 7px;
  font-size: 12px; color: var(--ink-soft); text-align: left;
}
.notif-detail-row svg { width: 13px; height: 13px; flex-shrink: 0; }

.modal-enter-active { transition: opacity .2s cubic-bezier(.2,.7,.2,1); }
.modal-leave-active { transition: opacity .15s cubic-bezier(.2,.7,.2,1); }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
