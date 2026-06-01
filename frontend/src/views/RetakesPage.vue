<script setup>
import { ref, computed, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { useAuthStore } from '../stores/auth'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { fmtDate, fmtTime, partsInTZ, toUtcISO } from '../utils/datetime'

const auth   = useAuthStore()
const router = useRouter()
const sidebarOpen = ref(false)

// ── Data ──────────────────────────────────────────────────
const retakes  = ref([])
const discMap  = ref({})
const loading  = ref(true)
const loadErr  = ref('')

// ── Filters ───────────────────────────────────────────────
const search       = ref('')
const statusFilter = ref('all')

const STATUS_LABELS = {
  scheduled:   'Запланирована',
  in_progress: 'Идёт',
  completed:   'Завершена',
  cancelled:   'Отменена',
}
const KIND_LABELS = { normal: 'Обычная', commission: 'С комиссией' }

const CHIPS = [
  { value: 'all',         label: 'Все' },
  { value: 'scheduled',   label: 'Запланированные' },
  { value: 'in_progress', label: 'Идут сейчас' },
  { value: 'completed',   label: 'Завершённые' },
  { value: 'cancelled',   label: 'Отменённые' },
]

const filteredRetakes = computed(() => {
  let list = retakes.value
  if (statusFilter.value !== 'all') {
    list = list.filter(r => r.status === statusFilter.value)
  }
  const q = search.value.trim().toLowerCase()
  if (q) {
    list = list.filter(r => {
      const disc = (discMap.value[r.discipline_id] || '').toLowerCase()
      return disc.includes(q) || (r.building || '').toLowerCase().includes(q) || (r.room || '').toLowerCase().includes(q)
    })
  }
  return list
})

const counts = computed(() => {
  const m = {}
  for (const c of CHIPS) {
    m[c.value] = c.value === 'all'
      ? retakes.value.length
      : retakes.value.filter(r => r.status === c.value).length
  }
  return m
})

// ── Load ──────────────────────────────────────────────────
onMounted(async () => {
  try {
    const [discRes, retakesRes] = await Promise.allSettled([
      disciplinesApi.getAll({ limit: 500 }),
      auth.isDean ? retakesApi.getAll() : retakesApi.getMy(),
    ])
    if (discRes.status === 'fulfilled') {
      const items = discRes.value.data.items ?? discRes.value.data ?? []
      items.forEach(d => { discMap.value[d.id] = d.name || d.code })
    }
    if (retakesRes.status === 'fulfilled') {
      retakes.value = retakesRes.value.data.items ?? retakesRes.value.data ?? []
    } else {
      loadErr.value = 'Не удалось загрузить пересдачи'
    }
  } finally {
    loading.value = false
  }
})

// fmtDate / fmtTime импортированы из utils/datetime — форматируют в зоне
// вуза (Ханты, UTC+5), а не в зоне устройства.

// ── Teacher action ────────────────────────────────────────
function goTeacherEdit(retake) {
  router.push({ path: '/teacher-requests', query: { retakeId: retake.id } })
}

// ── Dean edit modal ───────────────────────────────────────
const editOpen   = ref(false)
const editTarget = ref(null)
const editForm   = reactive({ building: '', room: '', hour: '09', minute: '00', duration: 90 })
const editSaving = ref(false)
const editError  = ref('')

const STEP = 5, DMIN = 15, DMAX = 480
function clamp(v, mn, mx) { return Math.max(mn, Math.min(mx, v || mn)) }

function openEdit(r) {
  editTarget.value = r
  editForm.building = r.building || ''
  editForm.room     = r.room     || ''
  editForm.duration = r.duration_minutes || 90
  if (r.scheduled_at) {
    // Предзаполняем часы/минуты временем вуза, а не зоной устройства.
    const p = partsInTZ(r.scheduled_at)
    editForm.hour   = p.hour
    editForm.minute = p.minute
  }
  editError.value = ''
  editOpen.value  = true
}
function closeEdit() { editOpen.value = false; editTarget.value = null }

function onHourBlur()   { editForm.hour   = String(clamp(parseInt(editForm.hour,   10), 0, 23)).padStart(2, '0') }
function onMinuteBlur() { editForm.minute = String(clamp(parseInt(editForm.minute, 10), 0, 59)).padStart(2, '0') }
function onTimeKey(e)   { e.target.value = e.target.value.replace(/\D/g, '').slice(0, 2) }

async function saveEdit() {
  if (!editTarget.value) return
  editError.value = ''
  editSaving.value = true
  try {
    // Дата берётся из исходной пересдачи (в зоне вуза), часы/минуты —
    // из формы. Собираем UTC-ISO явно через зону вуза.
    const p = partsInTZ(editTarget.value.scheduled_at)
    const newScheduledAt = toUtcISO(
      `${p.year}-${p.month}-${p.day}`,
      `${editForm.hour}:${editForm.minute}`,
    )
    await retakesApi.update(editTarget.value.id, {
      building:         editForm.building,
      room:             editForm.room,
      duration_minutes: editForm.duration,
      scheduled_at:     newScheduledAt,
    })
    const idx = retakes.value.findIndex(r => r.id === editTarget.value.id)
    if (idx !== -1) {
      retakes.value[idx] = {
        ...retakes.value[idx],
        building:         editForm.building,
        room:             editForm.room,
        duration_minutes: editForm.duration,
        scheduled_at:     newScheduledAt,
      }
    }
    closeEdit()
  } catch {
    editError.value = 'Ошибка при сохранении'
  } finally {
    editSaving.value = false
  }
}
</script>

<template>
  <div class="retakes-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-wrap">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">

        <!-- ── Page head ─────────────────────────────── -->
        <div class="page-head">
          <div class="head-left">
            <h1 class="page-title">Пересдачи</h1>
            <span class="total-badge">{{ retakes.length }}</span>
          </div>
          <div class="search-wrap">
            <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
            </svg>
            <input class="search-input" v-model="search" placeholder="Поиск по дисциплине, корпусу…" />
          </div>
        </div>

        <!-- ── Status chips ───────────────────────────── -->
        <div class="chips-row">
          <button
            v-for="c in CHIPS" :key="c.value"
            class="chip"
            :class="{ active: statusFilter === c.value, ['chip-' + c.value]: true }"
            @click="statusFilter = c.value"
          >
            {{ c.label }}
            <span class="chip-count">{{ counts[c.value] }}</span>
          </button>
        </div>

        <!-- ── Loading / Error ────────────────────────── -->
        <div v-if="loading" class="state-center">
          <div class="spinner" />
          <span>Загрузка пересдач…</span>
        </div>

        <div v-else-if="loadErr" class="state-center state-error">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
          {{ loadErr }}
        </div>

        <!-- ── Empty ──────────────────────────────────── -->
        <div v-else-if="filteredRetakes.length === 0" class="state-center state-empty">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
            <path d="M23 4v6h-6"/><path d="M1 20v-6h6"/>
            <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/>
          </svg>
          <span>{{ search || statusFilter !== 'all' ? 'Ничего не найдено' : 'Пересдач пока нет' }}</span>
        </div>

        <!-- ── Cards grid ─────────────────────────────── -->
        <div v-else class="cards-grid">
          <div v-for="r in filteredRetakes" :key="r.id" class="retake-card" :class="'card-' + r.status">

            <!-- Card accent bar -->
            <div class="card-accent" />

            <!-- Head -->
            <div class="card-head">
              <div class="card-disc">{{ discMap[r.discipline_id] || 'Дисциплина' }}</div>
              <span class="status-chip" :class="'s-' + r.status">{{ STATUS_LABELS[r.status] || r.status }}</span>
            </div>

            <!-- Meta row -->
            <div class="card-meta">
              <div class="meta-item">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                  <rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/>
                </svg>
                <span>{{ fmtDate(r.scheduled_at) }}</span>
              </div>
              <div class="meta-item">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                  <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
                </svg>
                <span>{{ fmtTime(r.scheduled_at) }}{{ r.duration_minutes ? ' · ' + r.duration_minutes + ' мин' : '' }}</span>
              </div>
              <div v-if="r.building || r.room" class="meta-item">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                  <path d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>
                </svg>
                <span>{{ [r.building && ('Корп. ' + r.building), r.room && ('Ауд. ' + r.room)].filter(Boolean).join(', ') || '—' }}</span>
              </div>
            </div>

            <!-- Footer -->
            <div class="card-foot">
              <span class="kind-tag">{{ KIND_LABELS[r.kind] || r.kind }}</span>
              <div class="card-actions">
                <!-- Teacher: request edit -->
                <button v-if="auth.isTeacher" class="btn-action" @click="goTeacherEdit(r)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                    <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                    <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                  </svg>
                  Изменить
                </button>
                <!-- Dean: direct edit -->
                <button v-if="auth.isDean" class="btn-action" @click="openEdit(r)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                    <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                    <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                  </svg>
                  Изменить
                </button>
              </div>
            </div>

          </div>
        </div>

      </main>
    </div>

    <!-- ── Dean edit modal ───────────────────────────── -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="editOpen" class="modal-overlay" @click.self="closeEdit">
          <div class="modal">

            <div class="modal-head">
              <span class="modal-title">Редактировать пересдачу</span>
              <button class="modal-close" @click="closeEdit" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
            </div>

            <div class="modal-body">
              <div class="modal-disc">{{ discMap[editTarget?.discipline_id] || 'Дисциплина' }}</div>

              <div class="form-row">
                <div class="field">
                  <label>Время</label>
                  <div class="time-picker">
                    <input class="time-input" type="text" inputmode="numeric" v-model="editForm.hour"
                           maxlength="2" placeholder="09" @input="onTimeKey" @blur="onHourBlur" />
                    <span class="time-colon">:</span>
                    <input class="time-input" type="text" inputmode="numeric" v-model="editForm.minute"
                           maxlength="2" placeholder="00" @input="onTimeKey" @blur="onMinuteBlur" />
                  </div>
                </div>
                <div class="field">
                  <label>Длительность (мин)</label>
                  <div class="stepper">
                    <button type="button" class="stepper-btn" @click="editForm.duration = clamp(editForm.duration - STEP, DMIN, DMAX)" :disabled="editForm.duration <= DMIN">−</button>
                    <input class="stepper-input" type="number" v-model.number="editForm.duration" min="15" max="480" @blur="editForm.duration = clamp(editForm.duration, DMIN, DMAX)" />
                    <button type="button" class="stepper-btn" @click="editForm.duration = clamp(editForm.duration + STEP, DMIN, DMAX)" :disabled="editForm.duration >= DMAX">+</button>
                  </div>
                </div>
              </div>

              <div class="form-row">
                <div class="field">
                  <label>Корпус</label>
                  <input class="input" v-model="editForm.building" placeholder="№ корпуса" />
                </div>
                <div class="field">
                  <label>Аудитория</label>
                  <input class="input" v-model="editForm.room" placeholder="№ аудитории" />
                </div>
              </div>

              <p v-if="editError" class="form-error">{{ editError }}</p>
            </div>

            <div class="modal-foot">
              <button class="btn-cancel" @click="closeEdit">Отмена</button>
              <button class="btn-save" :disabled="editSaving" @click="saveEdit">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M20 6L9 17l-5-5"/>
                </svg>
                {{ editSaving ? 'Сохранение…' : 'Сохранить' }}
              </button>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

.retakes-root {
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

.page-wrap { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }
.main      { flex: 1; padding: 24px; display: flex; flex-direction: column; gap: 20px; }

/* ── Page head ── */
.page-head {
  display: flex; align-items: center; justify-content: space-between;
  gap: 16px; flex-wrap: wrap;
}
.head-left { display: flex; align-items: center; gap: 10px; }
.page-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 22px; font-weight: 700; color: #3C38B6; margin: 0;
}
.total-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 28px; height: 24px; padding: 0 8px;
  background: rgba(59,63,224,.1); color: var(--brand-ink);
  border-radius: 20px; font: 600 13px/1 'Inter', sans-serif;
}

.search-wrap {
  position: relative; flex: 1; min-width: 200px; max-width: 340px;
}
.search-icon {
  position: absolute; left: 12px; top: 50%; transform: translateY(-50%);
  width: 16px; height: 16px; color: var(--ink-soft); pointer-events: none;
}
.search-input {
  appearance: none; width: 100%; height: 40px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: var(--card); padding: 0 12px 0 38px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.search-input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.search-input::placeholder { color: #b7b9c2; }

/* ── Chips ── */
.chips-row {
  display: flex; gap: 8px; flex-wrap: wrap;
}
.chip {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 0 14px; height: 34px; border-radius: 20px;
  border: 1.5px solid var(--line); background: var(--card);
  font: 500 13px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; transition: all .15s var(--ease);
  white-space: nowrap;
}
.chip:hover { border-color: var(--brand); color: var(--brand); }
.chip.active {
  border-color: var(--brand); background: var(--brand); color: #fff;
  box-shadow: 0 4px 14px -4px rgba(59,63,224,.4);
}
.chip-count {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 18px; height: 18px; padding: 0 4px; border-radius: 10px;
  background: rgba(0,0,0,.1); font: 700 11px/1 'Inter', sans-serif;
}
.chip.active .chip-count { background: rgba(255,255,255,.25); }

/* ── States ── */
.state-center {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; padding: 60px 20px;
  font: 500 15px/1.5 'Inter', sans-serif; color: var(--ink-soft); text-align: center;
}
.state-empty svg, .state-error svg { width: 48px; height: 48px; opacity: .4; }
.state-error { color: #dc2626; }
.spinner {
  width: 32px; height: 32px; border-radius: 50%;
  border: 3px solid var(--line); border-top-color: var(--brand);
  animation: spin .8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg) } }

/* ── Cards grid ── */
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

/* ── Retake card ── */
.retake-card {
  background: var(--card); border-radius: 12px;
  box-shadow: var(--shadow); overflow: hidden;
  display: flex; flex-direction: column;
  transition: box-shadow .2s var(--ease), transform .2s var(--ease);
  position: relative;
}
.retake-card:hover { box-shadow: 0 6px 24px rgba(20,22,60,.12); transform: translateY(-1px); }

.card-accent {
  position: absolute; left: 0; top: 0; bottom: 0; width: 4px;
}
.card-scheduled   .card-accent { background: #3b3fe0; }
.card-in_progress .card-accent { background: #f59e0b; }
.card-completed   .card-accent { background: #10b981; }
.card-cancelled   .card-accent { background: #9ca3af; }

.card-head {
  display: flex; align-items: flex-start; justify-content: space-between; gap: 10px;
  padding: 16px 16px 10px 20px;
}
.card-disc {
  font: 600 14px/1.4 'Inter', sans-serif; color: var(--ink); flex: 1; min-width: 0;
}

/* ── Status chips ── */
.status-chip {
  flex-shrink: 0; padding: 3px 10px; border-radius: 20px;
  font: 600 11px/1.4 'Inter', sans-serif; white-space: nowrap;
}
.s-scheduled   { background: rgba(59,63,224,.1);   color: var(--brand-ink); }
.s-in_progress { background: rgba(245,158,11,.12); color: #b45309; }
.s-completed   { background: rgba(16,185,129,.12); color: #065f46; }
.s-cancelled   { background: rgba(156,163,175,.15); color: #4b5563; }

/* ── Meta ── */
.card-meta {
  display: flex; flex-direction: column; gap: 7px;
  padding: 4px 16px 12px 20px;
}
.meta-item {
  display: flex; align-items: center; gap: 8px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink-soft);
}
.meta-item svg { width: 14px; height: 14px; flex-shrink: 0; }

/* ── Footer ── */
.card-foot {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 16px 14px 20px;
  border-top: 1px solid var(--line);
  margin-top: auto;
}
.kind-tag {
  font: 500 12px/1 'Inter', sans-serif; color: var(--ink-soft);
}
.card-actions { display: flex; gap: 8px; }

.btn-action {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 0 14px; height: 32px; border: none; border-radius: 8px;
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 12px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 4px 12px -4px rgba(91,59,217,.5);
  transition: transform .15s var(--ease), box-shadow .15s var(--ease);
}
.btn-action:hover { transform: scale(1.04); box-shadow: 0 4px 10px -4px rgba(91,59,217,.65); }
.btn-action svg { width: 13px; height: 13px; }

/* ── Modal ── */
.modal-overlay {
  position: fixed; inset: 0; z-index: 400;
  background: rgba(10,12,30,.5); backdrop-filter: blur(3px); -webkit-backdrop-filter: blur(3px);
  display: flex; align-items: center; justify-content: center; padding: 20px;
}
.modal {
  background: var(--card); border-radius: 16px; width: 100%; max-width: 480px;
  box-shadow: 0 24px 64px -12px rgba(10,12,30,.3);
  display: flex; flex-direction: column; max-height: 90vh; overflow: hidden;
}

.modal-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 20px; border-bottom: 1px solid var(--line); flex-shrink: 0;
}
.modal-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 700; color: #3C38B6;
}
.modal-close {
  width: 32px; height: 32px; border-radius: 8px;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s, color .15s;
}
.modal-close:hover { background: var(--bg); color: var(--ink); }
.modal-close svg { width: 16px; height: 16px; }

.modal-body { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 16px; }
.modal-body::-webkit-scrollbar { width: 4px; }
.modal-body::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }

.modal-disc {
  font: 600 14px/1.4 'Inter', sans-serif; color: var(--ink);
  background: var(--bg); border-radius: 8px; padding: 10px 14px;
}

.form-row { display: flex; gap: 16px; }
.field    { flex: 1; display: flex; flex-direction: column; gap: 6px; }
.field label { font: 500 13px/1 'Inter', sans-serif; color: var(--ink); }

.input {
  appearance: none; height: 40px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.input::placeholder { color: #b7b9c2; }

.time-picker { display: flex; align-items: center; gap: 6px; }
.time-input {
  height: 40px; width: 64px; text-align: center;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 8px;
  font: 14px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
  -moz-appearance: textfield;
}
.time-input::-webkit-outer-spin-button,
.time-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.time-input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.time-colon { font-size: 18px; font-weight: 700; color: var(--ink-soft); user-select: none; }

.stepper {
  display: flex; align-items: stretch; height: 40px;
  border: 1.5px solid var(--line); border-radius: var(--radius); overflow: hidden; background: #fff;
}
.stepper-btn {
  width: 38px; flex-shrink: 0; background: none; border: none;
  font-size: 20px; color: var(--ink-soft); cursor: pointer;
  display: grid; place-items: center; transition: background .15s, color .15s;
}
.stepper-btn:hover:not(:disabled) { background: rgba(59,63,224,.07); color: var(--brand); }
.stepper-btn:disabled { opacity: .35; cursor: not-allowed; }
.stepper-input {
  flex: 1; border: none; border-left: 1px solid var(--line); border-right: 1px solid var(--line);
  background: transparent; outline: none;
  font: 500 13px/1 'Inter', sans-serif; color: var(--ink); text-align: center;
  -moz-appearance: textfield;
}
.stepper-input::-webkit-outer-spin-button,
.stepper-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }

.form-error { font: 13px/1.4 'Inter', sans-serif; color: #dc2626; margin: 0; }

.modal-foot {
  display: flex; justify-content: flex-end; gap: 10px;
  padding: 14px 20px; border-top: 1px solid var(--line); flex-shrink: 0;
}
.btn-cancel {
  padding: 0 18px; height: 40px; background: #fff;
  border: 1.5px solid #c5c7d4; border-radius: var(--radius);
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer;
  box-shadow: 0 1px 4px rgba(20,22,60,.08);
  transition: border-color .15s, color .15s;
}
.btn-cancel:hover { border-color: var(--brand); color: var(--brand); }
.btn-save {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 0 20px; height: 40px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 6px 18px -6px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-save:hover:not(:disabled) { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }
.btn-save:disabled { opacity: .6; cursor: not-allowed; }
.btn-save svg { width: 14px; height: 14px; }

/* ── Modal transition ── */
.modal-enter-active { transition: opacity .2s var(--ease); }
.modal-leave-active { transition: opacity .18s var(--ease); }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .modal { transition: transform .25s var(--ease); }
.modal-leave-active  .modal { transition: transform .2s  var(--ease); }
.modal-enter-from .modal, .modal-leave-to .modal { transform: scale(.96) translateY(12px); }

/* ── Responsive ── */
@media (max-width: 640px) {
  .main { padding: 16px; gap: 16px; }
  .page-head { flex-direction: column; align-items: stretch; }
  .search-wrap { max-width: 100%; }
  .cards-grid { grid-template-columns: 1fr; }
  .form-row { flex-direction: column; }
}
</style>
