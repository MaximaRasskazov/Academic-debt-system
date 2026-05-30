<script setup>
import { ref, computed, reactive, onMounted } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { useAuthStore } from '../stores/auth'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { usersApi } from '../api/users'
import { debtsApi } from '../api/debts'
import { changeRequestsApi } from '../api/changeRequests'
import VueDatePicker from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'

const auth = useAuthStore()
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
const KIND_LABELS = { regular: 'Обычная', commission: 'С комиссией' }

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

// ── Helpers ───────────────────────────────────────────────
function fmtDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: 'long', year: 'numeric' })
}
function fmtTime(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

// ── Teacher: запрос изменения уже назначенной пересдачи ───────
// По ТЗ: "Преподаватель может подать заявку на изменение времени и
// места пересдачи... Деканат должен рассмотреть эту заявку и
// одобрить/не одобрить её. В случае неодобрения необходимо написать
// причину."
//
// Под капотом — POST /api/retake-change-requests с теми полями, что
// препод хочет изменить. Бэк требует actor=участник этой пересдачи
// и retake.status ∈ {scheduled, in_progress}. Декан увидит заявку
// на /requests, таб "Изменения пересдач".
const reqChangeOpen   = ref(false)
const reqChangeTarget = ref(null)
const reqChangeForm   = reactive({
  scheduledAt: '',      // ISO без часов — отдельные поля даты+времени
  hour:        '',
  minute:      '',
  duration:    '',      // строка чтобы пустое = "не меняем"
  building:    '',
  room:        '',
  notes:       '',
  reason:      '',
})
const reqChangeSaving  = ref(false)
const reqChangeError   = ref('')
const reqChangeSuccess = ref(false)

function openRequestChange(retake) {
  reqChangeTarget.value = retake
  // НИЧЕГО не префилим — пустые поля означают "не менять". Препод
  // заполняет только то, что хочет изменить. Так бизнес-логика
  // соответствует семантике "requested_changes": каждое поле опционально.
  Object.assign(reqChangeForm, {
    scheduledAt: '', hour: '', minute: '',
    duration: '', building: '', room: '', notes: '', reason: '',
  })
  reqChangeError.value = ''
  reqChangeSuccess.value = false
  reqChangeOpen.value = true
}
function closeRequestChange() {
  reqChangeOpen.value = false
  reqChangeTarget.value = null
}

async function submitRequestChange() {
  reqChangeError.value = ''

  // Собираем requested_changes только из непустых полей.
  const changes = {}
  const f = reqChangeForm

  // Дата+время: если указана дата ИЛИ часы/минуты — собираем полный ISO.
  // Если только дата без времени — берём 00:00; если только время — ошибка.
  if (f.scheduledAt || f.hour || f.minute) {
    if (!f.scheduledAt) {
      reqChangeError.value = 'Укажите дату вместе с временем'
      return
    }
    const hh = String(f.hour || '00').padStart(2, '0')
    const mm = String(f.minute || '00').padStart(2, '0')
    // scheduledAt format dd.MM.yyyy → нужен ISO. VueDatePicker model-type="format"
    const [d, mo, y] = f.scheduledAt.split('.')
    if (!d || !mo || !y) {
      reqChangeError.value = 'Дата в формате дд.мм.гггг'
      return
    }
    const iso = new Date(`${y}-${mo}-${d}T${hh}:${mm}:00`).toISOString()
    changes.scheduled_at = iso
  }

  if (f.duration !== '' && f.duration != null) {
    const d = Number(f.duration)
    if (!Number.isInteger(d) || d <= 0) {
      reqChangeError.value = 'Длительность должна быть положительным числом минут'
      return
    }
    changes.duration_minutes = d
  }

  if (f.building.trim() !== '') changes.building = f.building.trim()
  if (f.room.trim()     !== '') changes.room     = f.room.trim()
  if (f.notes.trim()    !== '') changes.notes    = f.notes.trim()

  if (Object.keys(changes).length === 0) {
    reqChangeError.value = 'Укажите хотя бы одно изменение'
    return
  }

  reqChangeSaving.value = true
  try {
    await changeRequestsApi.submit({
      retake_id: reqChangeTarget.value.id,
      requested_changes: changes,
      reason: f.reason.trim() || undefined,
    })
    reqChangeSuccess.value = true
    setTimeout(() => closeRequestChange(), 1400)
  } catch (e) {
    reqChangeError.value = e.response?.data?.message
      || e.response?.data?.error
      || 'Не удалось отправить заявку'
  } finally {
    reqChangeSaving.value = false
  }
}

// ── Dean edit modal ───────────────────────────────────────
const editOpen        = ref(false)
const editTarget      = ref(null)
const editForm        = reactive({ building: '', room: '', hour: '09', minute: '00', duration: 90 })
const editSaving      = ref(false)
const editError       = ref('')

// Participants state
const editParticipants   = ref({ teachers: [], students: [] })
const editLoadingPart    = ref(false)
const editAllStudents    = ref([])
const editAllTeachers    = ref([])
const editStudentSearch  = ref('')
const editTeacherSearch  = ref('')
const editStudentOpen    = ref(false)
const editTeacherOpen    = ref(false)
let editStudentBlur = null, editTeacherBlur = null

const editFilteredStudents = computed(() => {
  const q = editStudentSearch.value.toLowerCase()
  const enrolled = new Set(editParticipants.value.students.map(s => s.user_id))
  return editAllStudents.value
    .filter(s => !enrolled.has(s.id) && (s.name.toLowerCase().includes(q) || (s.group || '').toLowerCase().includes(q)))
    .slice(0, 8)
})
const editFilteredTeachers = computed(() => {
  const q = editTeacherSearch.value.toLowerCase()
  const enrolled = new Set(editParticipants.value.teachers.map(t => t.user_id))
  return editAllTeachers.value
    .filter(t => !enrolled.has(t.id) && t.name.toLowerCase().includes(q))
    .slice(0, 8)
})

// Status change
const STATUS_TRANSITIONS = {
  scheduled:   [{ value: 'in_progress', label: 'Начать пересдачу' }, { value: 'cancelled', label: 'Отменить' }],
  in_progress: [{ value: 'completed',   label: 'Завершить' },         { value: 'cancelled', label: 'Отменить' }],
}
const availableStatuses = computed(() => STATUS_TRANSITIONS[editTarget.value?.status] ?? [])
const statusSaving = ref(false)

async function changeStatus(newStatus) {
  if (!editTarget.value) return
  statusSaving.value = true
  editError.value = ''
  try {
    if (newStatus === 'in_progress') await retakesApi.start(editTarget.value.id)
    else if (newStatus === 'completed') await retakesApi.complete(editTarget.value.id)
    else if (newStatus === 'cancelled') await retakesApi.cancel(editTarget.value.id)
    const idx = retakes.value.findIndex(r => r.id === editTarget.value.id)
    if (idx !== -1) retakes.value[idx] = { ...retakes.value[idx], status: newStatus }
    editTarget.value = { ...editTarget.value, status: newStatus }
  } catch {
    editError.value = 'Ошибка при изменении статуса'
  } finally {
    statusSaving.value = false
  }
}

const STEP = 5, DMIN = 15, DMAX = 480
function clamp(v, mn, mx) { return Math.max(mn, Math.min(mx, v || mn)) }

async function openEdit(r) {
  editTarget.value = r
  editForm.building = r.building || ''
  editForm.room     = r.room     || ''
  editForm.duration = r.duration_minutes || 90
  if (r.scheduled_at) {
    const d = new Date(r.scheduled_at)
    editForm.hour   = String(d.getHours()).padStart(2, '0')
    editForm.minute = String(d.getMinutes()).padStart(2, '0')
  }
  editError.value = ''
  editParticipants.value = { teachers: [], students: [] }
  editAllStudents.value = []
  editAllTeachers.value = []
  editOpen.value = true

  editLoadingPart.value = true
  try {
    const [partRes, studRes, teachRes, debtsRes] = await Promise.allSettled([
      retakesApi.getParticipants(r.id),
      usersApi.getAll({ role: 'student', limit: 500 }),
      usersApi.getAll({ role: 'teacher', limit: 200 }),
      debtsApi.getAll({ limit: 500 }),
    ])
    if (partRes.status === 'fulfilled') {
      const parts = partRes.value.data ?? []
      editParticipants.value = {
        teachers: parts.filter(p => p.kind === 'teacher' || p.kind === 'commission_member'),
        students: parts.filter(p => p.kind === 'student'),
      }
    }
    const debtMap = {}
    if (debtsRes.status === 'fulfilled') {
      for (const d of debtsRes.value.data.items ?? []) {
        if (d.discipline_id === r.discipline_id && d.status === 'open') debtMap[d.student_id] = d.id
      }
    }
    if (studRes.status === 'fulfilled') {
      editAllStudents.value = (studRes.value.data.items ?? [])
        .filter(u => debtMap[u.id])
        .map(u => ({ id: u.id, name: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' '), group: u.group_name ?? '', debtId: debtMap[u.id] }))
    }
    if (teachRes.status === 'fulfilled') {
      editAllTeachers.value = (teachRes.value.data.items ?? [])
        .map(u => ({ id: u.id, name: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ') }))
    }
  } finally {
    editLoadingPart.value = false
  }
}

function closeEdit() { editOpen.value = false; editTarget.value = null }

function onHourBlur()   { editForm.hour   = String(clamp(parseInt(editForm.hour,   10), 0, 23)).padStart(2, '0') }
function onMinuteBlur() { editForm.minute = String(clamp(parseInt(editForm.minute, 10), 0, 59)).padStart(2, '0') }
function onTimeKey(e)   { e.target.value = e.target.value.replace(/\D/g, '').slice(0, 2) }

async function removeParticipant(type, userId) {
  try {
    if (type === 'teacher') {
      await retakesApi.removeTeacher(editTarget.value.id, userId)
      editParticipants.value.teachers = editParticipants.value.teachers.filter(p => p.user_id !== userId)
    } else {
      await retakesApi.removeStudent(editTarget.value.id, userId)
      editParticipants.value.students = editParticipants.value.students.filter(p => p.user_id !== userId)
    }
  } catch { editError.value = 'Ошибка при удалении участника' }
}

async function addParticipant(type, user) {
  try {
    if (type === 'teacher') {
      await retakesApi.addTeacher(editTarget.value.id, user.id)
      editParticipants.value.teachers.push({ user_id: user.id, first_name: '', last_name: user.name, kind: 'teacher' })
      editTeacherSearch.value = ''
      editTeacherOpen.value = false
    } else {
      await retakesApi.addStudent(editTarget.value.id, { student_id: user.id, debt_id: user.debtId })
      editParticipants.value.students.push({ user_id: user.id, first_name: '', last_name: user.name, group: user.group, kind: 'student' })
      editStudentSearch.value = ''
      editStudentOpen.value = false
    }
  } catch { editError.value = 'Ошибка при добавлении участника' }
}

function participantName(p) {
  if (p.name) return p.name
  return [p.last_name, p.first_name, p.middle_name].filter(Boolean).join(' ') || '—'
}

async function saveEdit() {
  if (!editTarget.value) return
  editError.value = ''
  editSaving.value = true
  try {
    const base = new Date(editTarget.value.scheduled_at || new Date())
    base.setHours(parseInt(editForm.hour, 10), parseInt(editForm.minute, 10), 0, 0)
    await retakesApi.update(editTarget.value.id, {
      building:         editForm.building,
      room:             editForm.room,
      duration_minutes: editForm.duration,
      scheduled_at:     base.toISOString(),
    })
    const idx = retakes.value.findIndex(r => r.id === editTarget.value.id)
    if (idx !== -1) {
      retakes.value[idx] = { ...retakes.value[idx], building: editForm.building, room: editForm.room, duration_minutes: editForm.duration, scheduled_at: base.toISOString() }
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
                <!-- Teacher: запросить изменения (POST /api/retake-change-requests) -->
                <button v-if="auth.isTeacher && r.status !== 'completed' && r.status !== 'cancelled'"
                        class="btn-action" @click="openRequestChange(r)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                    <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                    <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                  </svg>
                  Запросить изменения
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

              <!-- Изменение статуса -->
              <div v-if="availableStatuses.length" class="modal-section">
                <div class="section-label">Статус</div>
                <div class="status-current">
                  Сейчас: <span class="status-chip" :class="'s-' + editTarget?.status">{{ STATUS_LABELS[editTarget?.status] }}</span>
                </div>
                <div class="status-actions">
                  <button
                    v-for="s in availableStatuses" :key="s.value"
                    class="btn-status" :class="'btn-status--' + s.value"
                    :disabled="statusSaving"
                    @click="changeStatus(s.value)"
                  >{{ s.label }}</button>
                </div>
              </div>

              <!-- Время + Длительность -->
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

              <!-- Корпус + Аудитория -->
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

              <!-- Преподаватели -->
              <div v-if="auth.isDean" class="modal-section">
                <div class="section-label">Преподаватели</div>
                <div v-if="editLoadingPart" class="part-loading">Загрузка…</div>
                <template v-else>
                  <div class="part-tags" v-if="editParticipants.teachers.length">
                    <span v-for="t in editParticipants.teachers" :key="t.user_id" class="part-tag">
                      {{ participantName(t) }}
                      <button type="button" class="tag-remove" @click="removeParticipant('teacher', t.user_id)">×</button>
                    </span>
                  </div>
                  <div class="picker-wrap-modal">
                    <input
                      class="input" v-model="editTeacherSearch" placeholder="Добавить преподавателя…"
                      @focus="editTeacherOpen = true"
                      @blur="editTeacherBlur = setTimeout(() => editTeacherOpen = false, 200)"
                      @keydown.enter.prevent="editFilteredTeachers.length && addParticipant('teacher', editFilteredTeachers[0])"
                    />
                    <div v-show="editTeacherOpen && editFilteredTeachers.length" class="picker-dropdown-modal">
                      <button v-for="t in editFilteredTeachers" :key="t.id"
                        type="button" class="picker-opt"
                        @mousedown.prevent="addParticipant('teacher', t)">{{ t.name }}</button>
                    </div>
                  </div>
                </template>
              </div>

              <!-- Студенты -->
              <div v-if="auth.isDean" class="modal-section">
                <div class="section-label">Студенты</div>
                <div v-if="editLoadingPart" class="part-loading">Загрузка…</div>
                <template v-else>
                  <div class="part-tags" v-if="editParticipants.students.length">
                    <span v-for="s in editParticipants.students" :key="s.user_id" class="part-tag part-tag--student">
                      {{ participantName(s) }}
                      <button type="button" class="tag-remove" @click="removeParticipant('student', s.user_id)">×</button>
                    </span>
                  </div>
                  <div class="picker-wrap-modal">
                    <input
                      class="input" v-model="editStudentSearch" placeholder="Добавить студента…"
                      @focus="editStudentOpen = true"
                      @blur="editStudentBlur = setTimeout(() => editStudentOpen = false, 200)"
                      @keydown.enter.prevent="editFilteredStudents.length && addParticipant('student', editFilteredStudents[0])"
                    />
                    <div v-show="editStudentOpen && editFilteredStudents.length" class="picker-dropdown-modal">
                      <button v-for="s in editFilteredStudents" :key="s.id"
                        type="button" class="picker-opt"
                        @mousedown.prevent="addParticipant('student', s)">
                        <span>{{ s.name }}</span>
                        <span v-if="s.group" class="opt-group">{{ s.group }}</span>
                      </button>
                    </div>
                  </div>
                </template>
              </div>

              <p v-if="editError" class="form-error">{{ editError }}</p>
            </div>

            <div class="modal-foot">
              <button class="btn-cancel" @click="closeEdit">Закрыть</button>
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

    <!-- ── Request-change modal (teacher) ── -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="reqChangeOpen" class="modal-overlay" @click.self="closeRequestChange">
          <div class="modal modal--edit">

            <div class="modal-head">
              <span class="modal-title">Запросить изменения пересдачи</span>
              <button class="modal-close" @click="closeRequestChange" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
              </button>
            </div>

            <div class="modal-body modal-body--form">
              <p class="hint-info">
                Заполните только те поля, которые хотите изменить. Пустые поля останутся прежними.
                Декан рассмотрит заявку и одобрит или отклонит её.
              </p>

              <div v-if="reqChangeTarget" class="current-info">
                <div class="ci-row"><span class="ci-key">Сейчас:</span></div>
                <div class="ci-row">
                  <span class="ci-key">Дата и время</span>
                  <span class="ci-val">{{ fmtDate(reqChangeTarget.scheduled_at) }} в {{ fmtTime(reqChangeTarget.scheduled_at) }}</span>
                </div>
                <div class="ci-row">
                  <span class="ci-key">Место</span>
                  <span class="ci-val">{{ reqChangeTarget.building || '—' }} / {{ reqChangeTarget.room || '—' }}</span>
                </div>
                <div class="ci-row">
                  <span class="ci-key">Длительность</span>
                  <span class="ci-val">{{ reqChangeTarget.duration_minutes }} мин</span>
                </div>
              </div>

              <form class="req-form" @submit.prevent="submitRequestChange" novalidate>
                <div class="form-row">
                  <div class="field" style="flex:2">
                    <label>Новая дата</label>
                    <VueDatePicker v-model="reqChangeForm.scheduledAt" locale="ru"
                                   format="dd.MM.yyyy" model-type="format"
                                   :enable-time-picker="false" auto-apply
                                   placeholder="дд.мм.гггг" />
                  </div>
                  <div class="field">
                    <label>Новое время</label>
                    <div class="time-picker">
                      <input class="time-input" type="text" inputmode="numeric"
                             v-model="reqChangeForm.hour" maxlength="2" placeholder="чч"
                             @input="(e) => e.target.value = e.target.value.replace(/\D/g, '').slice(0,2)" />
                      <span class="time-colon">:</span>
                      <input class="time-input" type="text" inputmode="numeric"
                             v-model="reqChangeForm.minute" maxlength="2" placeholder="мм"
                             @input="(e) => e.target.value = e.target.value.replace(/\D/g, '').slice(0,2)" />
                    </div>
                  </div>
                </div>

                <div class="form-row">
                  <div class="field">
                    <label>Длительность (мин)</label>
                    <input class="input" type="number" min="15" max="480"
                           v-model="reqChangeForm.duration" placeholder="напр. 90" />
                  </div>
                  <div class="field">
                    <label>Корпус</label>
                    <input class="input" v-model="reqChangeForm.building" placeholder="напр. 1" />
                  </div>
                  <div class="field">
                    <label>Аудитория</label>
                    <input class="input" v-model="reqChangeForm.room" placeholder="напр. 204" />
                  </div>
                </div>

                <div class="form-row">
                  <div class="field field--full">
                    <label>Дополнительные заметки</label>
                    <input class="input" v-model="reqChangeForm.notes" placeholder="опционально" />
                  </div>
                </div>

                <div class="form-row">
                  <div class="field field--full">
                    <label>Причина изменения</label>
                    <textarea class="input textarea" rows="3"
                              v-model="reqChangeForm.reason"
                              placeholder="Объясните декану, почему нужны изменения — это поможет ему быстрее одобрить заявку" />
                  </div>
                </div>

                <p v-if="reqChangeError" class="form-msg form-error">{{ reqChangeError }}</p>
                <p v-if="reqChangeSuccess" class="form-msg form-success">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M20 6L9 17l-5-5"/></svg>
                  Заявка отправлена декану
                </p>
              </form>
            </div>

            <div class="modal-foot">
              <button class="btn-cancel" type="button" @click="closeRequestChange">Отмена</button>
              <button class="btn-save" type="button" :disabled="reqChangeSaving" @click="submitRequestChange">
                {{ reqChangeSaving ? 'Отправка…' : 'Отправить заявку' }}
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

/* ── Modal (global т.к. Teleport выносит за пределы scoped-дерева) ── */
:global(.modal-overlay) {
  position: fixed; inset: 0; z-index: 400;
  background: rgba(10,12,30,.5); backdrop-filter: blur(3px); -webkit-backdrop-filter: blur(3px);
  display: flex; align-items: center; justify-content: center; padding: 20px;
}
:global(.modal) {
  --card:     #ffffff;
  --bg:       #f3f4f7;
  --ink:      #1a1d24;
  --ink-soft: #6b7280;
  --line:     #d7d9e0;
  --brand:    #3b3fe0;
  --radius:   10px;
  --ease:     cubic-bezier(.2,.7,.2,1);
  background: #ffffff; border-radius: 16px; width: 100%; max-width: 520px;
  box-shadow: 0 24px 64px -12px rgba(10,12,30,.3);
  display: flex; flex-direction: column; max-height: 90vh; overflow: hidden;
  border: 1px solid #e5e7eb;
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
.form-msg { font: 13px/1.4 'Inter', sans-serif; margin: 0; display: flex; align-items: center; gap: 6px; }
.form-success { color: #059669; }
.form-success svg { width: 15px; height: 15px; }

.textarea { height: auto; padding: 10px 12px; resize: vertical; line-height: 1.5; }
.field--full { width: 100%; flex: 1 1 100%; }

/* Request-change modal helpers */
.req-form { display: flex; flex-direction: column; gap: 14px; }

.hint-info {
  background: rgba(59,63,224,.06);
  border: 1px solid rgba(59,63,224,.18);
  border-radius: var(--radius);
  padding: 10px 14px;
  font: 13px/1.45 'Inter', sans-serif;
  color: var(--brand-ink);
  margin: 0 0 16px;
}

.current-info {
  background: var(--bg);
  border-radius: var(--radius);
  padding: 12px 14px;
  display: flex; flex-direction: column; gap: 6px;
  margin-bottom: 16px;
}
.ci-row {
  display: flex; gap: 14px; align-items: baseline;
  font: 13px/1.4 'Inter', sans-serif;
}
.ci-key { color: var(--ink-soft); min-width: 130px; font-weight: 500; }
.ci-val { color: var(--ink); font-weight: 600; }

/* ── Modal sections (статус + участники) ── */
.modal-section {
  display: flex; flex-direction: column; gap: 8px;
  padding: 12px; background: var(--bg); border-radius: var(--radius);
}
.section-label { font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft); text-transform: uppercase; letter-spacing: .05em; }
.status-current { font: 13px/1.4 'Inter', sans-serif; color: var(--ink); display: flex; align-items: center; gap: 8px; }
.status-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.btn-status {
  padding: 6px 14px; border-radius: 8px; font: 600 12px/1 'Inter', sans-serif;
  cursor: pointer; border: 1.5px solid transparent; transition: all .15s;
}
.btn-status--in_progress { background: rgba(59,130,246,.1); color: #1d4ed8; border-color: rgba(59,130,246,.3); }
.btn-status--in_progress:hover { background: rgba(59,130,246,.2); }
.btn-status--completed   { background: rgba(16,185,129,.1); color: #065f46; border-color: rgba(16,185,129,.3); }
.btn-status--completed:hover { background: rgba(16,185,129,.2); }
.btn-status--cancelled   { background: rgba(239,68,68,.1); color: #991b1b; border-color: rgba(239,68,68,.3); }
.btn-status--cancelled:hover { background: rgba(239,68,68,.2); }
.btn-status:disabled { opacity: .5; cursor: not-allowed; }

.part-loading { font: 13px/1 'Inter', sans-serif; color: var(--ink-soft); }
.part-tags { display: flex; flex-wrap: wrap; gap: 6px; }
.part-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 10px 4px 12px; border-radius: 20px;
  font: 500 12px/1 'Inter', sans-serif;
  background: rgba(59,63,224,.1); color: #2a2e9e;
}
.part-tag--student { background: rgba(16,185,129,.1); color: #065f46; }
.tag-remove {
  background: none; border: none; cursor: pointer; color: inherit;
  font-size: 14px; line-height: 1; padding: 0 2px; opacity: .6; transition: opacity .15s;
}
.tag-remove:hover { opacity: 1; }

.picker-wrap-modal { position: relative; }
.picker-dropdown-modal {
  position: absolute; top: calc(100% + 2px); left: 0; right: 0; z-index: 200;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.14);
  max-height: 180px; overflow-y: auto;
}
.picker-dropdown-modal::-webkit-scrollbar { width: 4px; }
.picker-dropdown-modal::-webkit-scrollbar-track { background: transparent; }
.picker-dropdown-modal::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }
.picker-opt {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; text-align: left; background: none; border: none;
  padding: 8px 14px; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s; gap: 8px;
}
.picker-opt:hover { background: rgba(59,63,224,.07); }
.opt-group { font-size: 11px; color: var(--ink-soft); background: rgba(59,63,224,.08); border-radius: 10px; padding: 2px 8px; }

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
