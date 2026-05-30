<script setup>
import { ref, computed, reactive, watch, onMounted, onUnmounted } from 'vue'
import VueDatePicker from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'

import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'
import { debtsApi } from '../api/debts'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { usersApi } from '../api/users'

const sidebarOpen = ref(false)

// ── Stats ─────────────────────────────────────────────────────
const upcomingRetakes = ref([])
const stats = ref([
  { label: 'Академических долгов', value: '—', accent: '#e63c5a' },
  { label: 'Назначено пересдач',   value: '—', accent: '#3b3fe0' },
  { label: 'Проводится сейчас',    value: '—', accent: '#f59e0b' },
  { label: 'Завершено в месяце',   value: '—', accent: '#10b981' },
])

// ── Disciplines ───────────────────────────────────────────────
const disciplines = ref([])
const discMap = ref({})
const disciplineId = ref('')
const discSearch = ref('')
const discDropdownOpen = ref(false)

const selectedDiscipline = computed(() => disciplines.value.find(d => d.id === disciplineId.value))

const filteredDisciplines = computed(() => {
  const q = discSearch.value.toLowerCase()
  return q
    ? disciplines.value.filter(d => d.name.toLowerCase().includes(q) || d.code.toLowerCase().includes(q))
    : disciplines.value
})

function selectDiscipline(id) {
  disciplineId.value = id
  discDropdownOpen.value = false
  discSearch.value = ''
}

// ── Participants ──────────────────────────────────────────────
const availableStudents = ref([])
const availableTeachers = ref([])
const loadingParticipants = ref(false)

const selectedStudents = ref([])
const selectedTeachers = ref([])
const studentSearch = ref('')
const teacherSearch = ref('')
const teacherOpen = ref(false)
const studentOpen = ref(false)
const groupSelect = ref('')

const selectedStudentIds = computed(() => new Set(selectedStudents.value.map(s => s.id)))
const selectedTeacherIds = computed(() => new Set(selectedTeachers.value.map(t => t.id)))
const availableGroups = computed(() => [...new Set(availableStudents.value.map(s => s.group).filter(Boolean))])

const filteredStudents = computed(() => {
  const q = studentSearch.value.toLowerCase()
  return availableStudents.value
    .filter(s => !selectedStudentIds.value.has(s.id) && (s.name.toLowerCase().includes(q) || s.group.toLowerCase().includes(q)))
    .slice(0, 8)
})

const filteredTeachers = computed(() => {
  const q = teacherSearch.value.toLowerCase()
  return availableTeachers.value
    .filter(t => !selectedTeacherIds.value.has(t.id) && t.name.toLowerCase().includes(q))
    .slice(0, 8)
})

let teacherBlurTimer = null
let studentBlurTimer = null
function onTeacherBlur() { teacherBlurTimer = setTimeout(() => { teacherOpen.value = false }, 200) }
function onStudentBlur() { studentBlurTimer = setTimeout(() => { studentOpen.value = false }, 200) }
function onTeacherFocus() { clearTimeout(teacherBlurTimer); teacherOpen.value = true }
function onStudentFocus() { clearTimeout(studentBlurTimer); studentOpen.value = true }

watch(disciplineId, async (id) => {
  availableStudents.value = []
  availableTeachers.value = []
  selectedStudents.value = []
  selectedTeachers.value = []
  groupSelect.value = ''
  if (!id) return

  loadingParticipants.value = true
  try {
    const [studRes, teachRes, debtsRes] = await Promise.allSettled([
      usersApi.getAll({ role: 'student', limit: 500 }),
      usersApi.getAll({ role: 'teacher', limit: 100 }),
      debtsApi.getAll({ limit: 500 }),
    ])

    const debtMap = {}
    if (debtsRes.status === 'fulfilled') {
      for (const d of debtsRes.value.data.items ?? []) {
        if (d.discipline_id === id && d.status === 'open') debtMap[d.student_id] = d.id
      }
    }

    if (studRes.status === 'fulfilled') {
      availableStudents.value = (studRes.value.data.items ?? [])
        .filter(u => debtMap[u.id])
        .map(u => ({
          id: u.id,
          name: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' '),
          group: u.group_name ?? '',
          debtId: debtMap[u.id],
        }))
    }

    if (teachRes.status === 'fulfilled') {
      const teacherItems = teachRes.value.data.items ?? teachRes.value.data ?? []
      availableTeachers.value = teacherItems.map(u => ({
        id: u.id,
        name: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' '),
      }))
    }
  } finally {
    loadingParticipants.value = false
  }
})

function addStudent(s) {
  clearTimeout(studentBlurTimer)
  if (!selectedStudentIds.value.has(s.id)) selectedStudents.value.push(s)
  studentSearch.value = ''
  studentOpen.value = true
}

function addGroup(group) {
  for (const s of availableStudents.value) {
    if (s.group === group && !selectedStudentIds.value.has(s.id)) selectedStudents.value.push(s)
  }
}

function removeStudent(id) { selectedStudents.value = selectedStudents.value.filter(s => s.id !== id) }

function addTeacher(t) {
  clearTimeout(teacherBlurTimer)
  if (selectedTeacherIds.value.has(t.id)) return
  if (!isCommission.value && selectedTeachers.value.length >= 1) return
  selectedTeachers.value.push(t)
  teacherSearch.value = ''
  teacherOpen.value = isCommission.value
}

function removeTeacher(id) { selectedTeachers.value = selectedTeachers.value.filter(t => t.id !== id) }

function onTeacherEnter() {
  if (filteredTeachers.value.length > 0) addTeacher(filteredTeachers.value[0])
}
function onStudentEnter() {
  if (filteredStudents.value.length > 0) addStudent(filteredStudents.value[0])
}

// ── Type ──────────────────────────────────────────────────────
const typeDropdownOpen = ref(false)
const typeOptions = [
  { value: 'normal',     label: 'Обычная' },
  { value: 'commission', label: 'С комиссией (мин. 3 преподавателя)' },
]
const retakeType = ref('normal')
function selectType(val) { retakeType.value = val; typeDropdownOpen.value = false }

const isCommission   = computed(() => retakeType.value === 'commission')
const minTeachers    = computed(() => isCommission.value ? 3 : 1)
const teacherCountOk = computed(() => selectedTeachers.value.length >= minTeachers.value)

watch(isCommission, (val) => {
  if (!val && selectedTeachers.value.length > 1) selectedTeachers.value.splice(1)
})

// ── Date / Time ───────────────────────────────────────────────
const retakeDate = ref(null)
const timeHour     = ref(9)
const timeMinute   = ref(0)
const hourDisplay   = ref('09')
const minuteDisplay = ref('00')
const timeString = computed(() =>
  `${String(timeHour.value).padStart(2, '0')}:${String(timeMinute.value).padStart(2, '0')}`
)

watch([timeHour, timeMinute], ([h, m]) => {
  hourDisplay.value = String(h).padStart(2, '0')
  minuteDisplay.value = String(m).padStart(2, '0')
}, { immediate: true })

function onTimeInput(e) { e.target.value = e.target.value.replace(/\D/g, '').slice(0, 2) }
function onHourBlur() {
  const n = Math.max(0, Math.min(23, parseInt(hourDisplay.value, 10) || 0))
  timeHour.value = n; hourDisplay.value = String(n).padStart(2, '0')
}
function onMinuteBlur() {
  const n = Math.max(0, Math.min(59, parseInt(minuteDisplay.value, 10) || 0))
  timeMinute.value = n; minuteDisplay.value = String(n).padStart(2, '0')
}

// ── Duration ──────────────────────────────────────────────────
const duration = ref(90)
const DURATION_STEP = 5, DURATION_MIN = 15, DURATION_MAX = 480
function decreaseDuration() { if (duration.value > DURATION_MIN) duration.value -= DURATION_STEP }
function increaseDuration()  { if (duration.value < DURATION_MAX) duration.value += DURATION_STEP }
function clampDuration() { duration.value = Math.max(DURATION_MIN, Math.min(DURATION_MAX, duration.value || DURATION_MIN)) }

// ── Building / Room ───────────────────────────────────────────
const building = ref('')
const room = ref('')

// ── Submit ────────────────────────────────────────────────────
const submitting  = ref(false)
const submitError   = ref('')
const submitSuccess = ref(false)

async function submitRetake() {
  submitError.value = ''
  if (!disciplineId.value)   { submitError.value = 'Выберите дисциплину'; return }
  if (!retakeDate.value)      { submitError.value = 'Укажите дату'; return }
  if (!building.value.trim()) { submitError.value = 'Укажите корпус'; return }
  if (!room.value.trim())     { submitError.value = 'Укажите аудиторию'; return }
  if (!teacherCountOk.value)  {
    submitError.value = isCommission.value
      ? 'Для комиссии нужно минимум 3 преподавателя'
      : 'Добавьте хотя бы одного преподавателя'
    return
  }

  const [day, month, year] = retakeDate.value.split('.')
  const scheduledAt = new Date(`${year}-${month}-${day}T${timeString.value}:00`).toISOString()

  submitting.value = true
  try {
    const { data: retake } = await retakesApi.create({
      discipline_id: disciplineId.value,
      kind: isCommission.value ? 'commission' : 'regular',
      building: building.value,
      room: room.value,
      scheduled_at: scheduledAt,
      duration_minutes: duration.value,
    })

    await Promise.all([
      ...selectedTeachers.value.map(t => retakesApi.addTeacher(retake.id, t.id)),
      ...selectedStudents.value.map(s => retakesApi.addStudent(retake.id, { student_id: s.id, debt_id: s.debtId })),
    ])

    disciplineId.value = ''
    retakeType.value = 'normal'
    retakeDate.value = null
    timeHour.value = 9; timeMinute.value = 0
    duration.value = 90
    building.value = ''; room.value = ''
    selectedStudents.value = []; selectedTeachers.value = []
    submitSuccess.value = true
    setTimeout(() => { submitSuccess.value = false }, 3000)
    await loadDashboard()
  } catch (e) {
    submitError.value = e.response?.data?.message || e.response?.data?.error || 'Ошибка при создании пересдачи'
  } finally {
    submitting.value = false
  }
}

// ── Outside click ─────────────────────────────────────────────
function handleOutsideClick(e) {
  if (!e.target.closest('.custom-select')) {
    typeDropdownOpen.value = false
    discDropdownOpen.value = false
  }
}

async function loadDashboard() {
  const [summaryRes, retakesRes, scheduledRes, disciplinesRes] = await Promise.allSettled([
    debtsApi.getSummary(),
    retakesApi.getAll({ limit: 200 }),
    retakesApi.getAll({ status: 'scheduled', limit: 10 }),
    disciplinesApi.getAll({ limit: 200 }),
  ])

  if (disciplinesRes.status === 'fulfilled') {
    disciplines.value = disciplinesRes.value.data.items ?? []
    for (const d of disciplines.value) discMap.value[d.id] = d.name || d.code
  }

  let totalDebts = 0
  if (summaryRes.status === 'fulfilled')
    totalDebts = (summaryRes.value.data ?? []).reduce((s, r) => s + (r.open_count ?? 0), 0)

  let totalRetakes = 0, inProgress = 0, completedMonth = 0
  if (retakesRes.status === 'fulfilled') {
    const items = retakesRes.value.data.items ?? []
    totalRetakes = retakesRes.value.data.total ?? items.length
    inProgress = items.filter(r => r.status === 'in_progress').length
    const now = new Date()
    completedMonth = items.filter(r => {
      if (r.status !== 'completed') return false
      const d = new Date(r.completed_at ?? r.updated_at ?? r.created_at)
      return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth()
    }).length
  }

  stats.value = [
    { label: 'Академических долгов', value: totalDebts,     accent: '#e63c5a' },
    { label: 'Назначено пересдач',   value: totalRetakes,   accent: '#3b3fe0' },
    { label: 'Проводится сейчас',    value: inProgress,     accent: '#f59e0b' },
    { label: 'Завершено в месяце',   value: completedMonth, accent: '#10b981' },
  ]

  if (scheduledRes.status === 'fulfilled') {
    upcomingRetakes.value = (scheduledRes.value.data.items ?? []).map(r => {
      const d = new Date(r.scheduled_at)
      return {
        id: r.id,
        subject: discMap.value[r.discipline_id] || 'Дисциплина',
        day: d.getDate(), month: d.getMonth() + 1, year: d.getFullYear(),
        time: `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`,
        building: r.building, room: r.room,
      }
    })
  }
}

onMounted(async () => {
  document.addEventListener('mousedown', handleOutsideClick)
  await loadDashboard()
})

onUnmounted(() => document.removeEventListener('mousedown', handleOutsideClick))
</script>

<template>
  <div class="dean-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-dean">

      <AppHeader @open-sidebar="sidebarOpen = true" />

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

              <!-- Дисциплина + Тип -->
              <div class="form-row">
                <div class="field">
                  <label>Дисциплина</label>
                  <div class="custom-select" :class="{ open: discDropdownOpen }">
                    <button type="button" class="custom-select-trigger" @click="discDropdownOpen = !discDropdownOpen">
                      <span :class="{ placeholder: !selectedDiscipline }">
                        {{ selectedDiscipline ? selectedDiscipline.name : 'Выберите дисциплину' }}
                      </span>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                        <path d="M6 9l6 6 6-6"/>
                      </svg>
                    </button>
                    <div class="custom-select-dropdown">
                      <div class="dropdown-search-wrap">
                        <input class="dropdown-search" v-model="discSearch" placeholder="Поиск..." @click.stop />
                      </div>
                      <div class="dropdown-scroll">
                        <button
                          v-for="d in filteredDisciplines" :key="d.id"
                          type="button" class="custom-select-option"
                          :class="{ selected: disciplineId === d.id }"
                          @click="selectDiscipline(d.id)"
                        >
                          <span class="disc-name">{{ d.name }}</span>
                          <span class="disc-code">{{ d.code }}</span>
                        </button>
                        <div v-if="!filteredDisciplines.length" class="dropdown-empty">Ничего не найдено</div>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="field">
                  <label>Тип пересдачи</label>
                  <div class="custom-select" :class="{ open: typeDropdownOpen }">
                    <button type="button" class="custom-select-trigger" @click="typeDropdownOpen = !typeDropdownOpen">
                      <span>{{ typeOptions.find(o => o.value === retakeType)?.label }}</span>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                        <path d="M6 9l6 6 6-6"/>
                      </svg>
                    </button>
                    <div class="custom-select-dropdown">
                      <button
                        v-for="opt in typeOptions" :key="opt.value"
                        type="button" class="custom-select-option"
                        :class="{ selected: retakeType === opt.value }"
                        @click="selectType(opt.value)"
                      >{{ opt.label }}</button>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Дата + Время + Длительность -->
              <div class="form-row">
                <div class="field">
                  <label>Дата</label>
                  <VueDatePicker
                    v-model="retakeDate"
                    locale="ru" format="dd.MM.yyyy" model-type="format"
                    :enable-time-picker="false" auto-apply placeholder="дд.мм.гггг"
                  />
                </div>
                <div class="field field--shrink">
                  <label>Время</label>
                  <div class="time-picker">
                    <input class="time-input" type="text" inputmode="numeric"
                      v-model="hourDisplay" maxlength="2" placeholder="00"
                      @input="onTimeInput" @blur="onHourBlur" />
                    <span class="time-colon">:</span>
                    <input class="time-input" type="text" inputmode="numeric"
                      v-model="minuteDisplay" maxlength="2" placeholder="00"
                      @input="onTimeInput" @blur="onMinuteBlur" />
                  </div>
                </div>
                <div class="field">
                  <label>Длительность (мин)</label>
                  <div class="stepper">
                    <button type="button" class="stepper-btn" @click="decreaseDuration" :disabled="duration <= DURATION_MIN">−</button>
                    <input class="stepper-input" type="number" v-model.number="duration" min="15" max="480" @blur="clampDuration" />
                    <button type="button" class="stepper-btn" @click="increaseDuration" :disabled="duration >= DURATION_MAX">+</button>
                  </div>
                </div>
              </div>

              <!-- Преподаватели -->
              <div class="form-row">
                <div class="field field--full picker-wrap">
                  <label>{{ isCommission ? 'Преподаватели (мин. 3)' : 'Преподаватель' }}</label>
                  <div
                    class="token-input"
                    :class="{ 'token-input--focused': teacherOpen, 'token-input--disabled': !disciplineId || loadingParticipants }"
                    @click="$el.querySelector('.token-field').focus()"
                  >
                    <span v-for="t in selectedTeachers" :key="t.id" class="token-chip">
                      <span class="token-chip-text">{{ t.name }}</span>
                      <button type="button" class="token-remove" @mousedown.prevent="removeTeacher(t.id)">×</button>
                    </span>
                    <input
                      class="token-field"
                      v-model="teacherSearch"
                      :placeholder="selectedTeachers.length ? '' : (loadingParticipants ? 'Загрузка...' : (disciplineId ? 'Введите имя...' : 'Сначала выберите дисциплину'))"
                      :disabled="!disciplineId || loadingParticipants"
                      @focus="onTeacherFocus"
                      @blur="onTeacherBlur"
                      @keydown.enter.prevent="onTeacherEnter"
                    />
                  </div>
                  <div v-show="teacherOpen && disciplineId" class="picker-dropdown">
                    <div v-if="filteredTeachers.length">
                      <button
                        v-for="t in filteredTeachers" :key="t.id"
                        type="button" class="picker-option"
                        @mousedown.prevent="addTeacher(t)"
                      >{{ t.name }}</button>
                    </div>
                    <div v-else class="picker-empty">
                      {{ loadingParticipants ? 'Загрузка...' : 'Нет совпадений' }}
                    </div>
                  </div>
                  <p v-if="isCommission && selectedTeachers.length > 0 && !teacherCountOk" class="field-hint-warn">
                    Для пересдачи с комиссией необходимо минимум 3 преподавателя
                  </p>
                </div>
              </div>

              <!-- Студенты + Группа -->
              <div class="form-row">
                <div class="field picker-wrap">
                  <label>Студенты</label>
                  <div
                    class="token-input"
                    :class="{ 'token-input--focused': studentOpen, 'token-input--disabled': !disciplineId || loadingParticipants }"
                    @click="$el.querySelector('.token-field-s').focus()"
                  >
                    <span v-for="s in selectedStudents" :key="s.id" class="token-chip token-chip--student">
                      <span class="token-chip-text">{{ s.name }}</span>
                      <span v-if="s.group" class="token-group">· {{ s.group }}</span>
                      <button type="button" class="token-remove" @mousedown.prevent="removeStudent(s.id)">×</button>
                    </span>
                    <input
                      class="token-field token-field-s"
                      v-model="studentSearch"
                      :placeholder="selectedStudents.length ? '' : (loadingParticipants ? 'Загрузка...' : (disciplineId ? 'Введите имя или группу...' : 'Сначала выберите дисциплину'))"
                      :disabled="!disciplineId || loadingParticipants"
                      @focus="onStudentFocus"
                      @blur="onStudentBlur"
                      @keydown.enter.prevent="onStudentEnter"
                    />
                  </div>
                  <div v-show="studentOpen && disciplineId" class="picker-dropdown">
                    <div v-if="filteredStudents.length">
                      <button
                        v-for="s in filteredStudents" :key="s.id"
                        type="button" class="picker-option"
                        @mousedown.prevent="addStudent(s)"
                      >
                        <span>{{ s.name }}</span>
                        <span v-if="s.group" class="suggest-group">{{ s.group }}</span>
                      </button>
                    </div>
                    <div v-else class="picker-empty">
                      {{ loadingParticipants ? 'Загрузка...' : (availableStudents.length ? 'Нет совпадений' : 'Нет студентов с долгами') }}
                    </div>
                  </div>
                </div>

                <div class="field">
                  <label>Группа</label>
                  <div class="group-select-row">
                    <select class="input" v-model="groupSelect" :disabled="!availableGroups.length">
                      <option value="">{{ disciplineId ? (availableGroups.length ? 'Выберите группу' : 'Нет групп') : 'Сначала выберите дисциплину' }}</option>
                      <option v-for="g in availableGroups" :key="g" :value="g">{{ g }}</option>
                    </select>
                    <button
                      type="button" class="btn-add-group"
                      :disabled="!groupSelect"
                      @click="addGroup(groupSelect); groupSelect = ''"
                    >+ Добавить</button>
                  </div>
                </div>
              </div>

              <!-- Корпус + Аудитория -->
              <div class="form-row">
                <div class="field">
                  <label>Корпус</label>
                  <input class="input" v-model="building" placeholder="№ корпуса" />
                </div>
                <div class="field">
                  <label>Аудитория</label>
                  <input class="input" v-model="room" placeholder="№ аудитории" />
                </div>
              </div>

              <p v-if="submitError"   class="submit-error">{{ submitError }}</p>
              <p v-if="submitSuccess" class="submit-success">Пересдача успешно создана</p>

              <button class="btn-primary" type="submit" :disabled="submitting">
                {{ submitting ? 'Создание…' : 'Назначить пересдачу' }}
              </button>
            </form>
          </div>

        </section>

        <!-- Правая колонка 30% -->
        <section class="col-right">
          <CalendarWidget :retakes="upcomingRetakes" />
          <UpcomingRetakes :retakes="upcomingRetakes" />
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

.page-dean { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }

.main {
  flex: 1; display: grid; grid-template-columns: 7fr 3fr;
  gap: 24px; padding: 24px; align-items: start;
}

/* ── Stats ── */
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
.stat-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 20px 20px 20px 24px;
  position: relative; overflow: hidden;
}
.stat-accent { position: absolute; left: 0; top: 0; bottom: 0; width: 4px; }
.stat-value  { font-size: 34px; font-weight: 700; line-height: 1; margin-bottom: 8px; }
.stat-label  { font-size: 12px; color: var(--ink-soft); line-height: 1.4; }

/* ── Section card ── */
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px; margin-bottom: 20px;
}
.section-card:last-child { margin-bottom: 0; }
.section-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0 0 20px;
}

/* ── Form ── */
.retake-form { display: flex; flex-direction: column; gap: 16px; }
.form-row    { display: flex; gap: 16px; }
.field       { flex: 1; display: flex; flex-direction: column; gap: 6px; }
.field--shrink { flex: none; }
.field--full   { width: 100%; }
.field label   { font-size: 13px; font-weight: 500; color: var(--ink); }

.input {
  appearance: none; height: 38px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.input::placeholder { color: #b7b9c2; }

/* ── Custom select ── */
.custom-select { position: relative; }
.custom-select-trigger {
  appearance: none; width: 100%; height: 38px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 36px 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink);
  display: flex; align-items: center; justify-content: space-between;
  cursor: pointer; text-align: left;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.custom-select-trigger:focus,
.custom-select.open .custom-select-trigger {
  border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); outline: none;
}
.custom-select-trigger svg {
  width: 16px; height: 16px; flex-shrink: 0; color: var(--ink-soft);
  transition: transform .2s var(--ease); position: absolute; right: 10px;
}
.custom-select.open .custom-select-trigger svg { transform: rotate(180deg); }
.custom-select-trigger .placeholder { color: #b7b9c2; }

.custom-select-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.12);
  z-index: 50; overflow: hidden;
  opacity: 0; pointer-events: none; transform: translateY(-4px);
  transition: opacity .15s var(--ease), transform .15s var(--ease);
}
.custom-select.open .custom-select-dropdown { opacity: 1; pointer-events: all; transform: translateY(0); }

.dropdown-search-wrap { padding: 8px 8px 4px; border-bottom: 1px solid var(--line); }
.dropdown-search {
  width: 100%; height: 30px; border: 1.5px solid var(--line); border-radius: 6px;
  padding: 0 10px; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  outline: none; background: #fff;
}
.dropdown-search:focus { border-color: var(--brand); }
.dropdown-scroll { max-height: 200px; overflow-y: auto; }
.dropdown-empty { padding: 10px 14px; font: 13px/1 'Inter', sans-serif; color: var(--ink-soft); }

.custom-select-option {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; text-align: left; background: none; border: none;
  padding: 9px 14px; font: 13px/1.4 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s; gap: 8px;
}
.custom-select-option:hover   { background: rgba(59,63,224,.06); }
.custom-select-option.selected { color: var(--brand); font-weight: 500; background: rgba(59,63,224,.05); }
.disc-name { flex: 1; }
.disc-code { font-size: 11px; color: var(--ink-soft); white-space: nowrap; }

/* ── Time picker ── */
.time-picker { display: flex; align-items: center; gap: 6px; }
.time-input {
  height: 38px; width: 64px; border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 8px; font: 14px/1 'Inter', sans-serif; color: var(--ink);
  text-align: center; outline: none; -moz-appearance: textfield;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.time-input::-webkit-outer-spin-button,
.time-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.time-input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.time-colon { font-size: 18px; font-weight: 700; color: var(--ink-soft); user-select: none; }

/* ── Stepper ── */
.stepper {
  display: flex; align-items: stretch; height: 38px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  overflow: hidden; background: #fff;
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
  background: transparent; outline: none; font: 13px/1 'Inter', sans-serif; font-weight: 500;
  color: var(--ink); text-align: center; -moz-appearance: textfield;
}
.stepper-input::-webkit-outer-spin-button,
.stepper-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }

/* ── Picker (teacher / student) ── */
.picker-wrap { position: relative; }
.picker-input-wrap { position: relative; }
.tags-row {
  display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 6px;
}
.teacher-tag {
  display: inline-flex; align-items: center; gap: 4px; padding: 4px 6px 4px 10px;
  background: rgba(59,63,224,.1); color: var(--brand-ink); border-radius: 20px;
  font-size: 12px; font-weight: 500;
}
.teacher-tag--student { background: rgba(16,185,129,.1); color: #065f46; }
.tag-group { opacity: .7; }
.teacher-tag-remove {
  background: none; border: none; cursor: pointer; color: inherit;
  font-size: 15px; line-height: 1; padding: 0 2px; opacity: .6; transition: opacity .15s;
}
.teacher-tag-remove:hover { opacity: 1; }

/* ── Token input (Bitrix-style chips inside field) ── */
.token-input {
  display: flex; flex-wrap: wrap; align-items: center; gap: 6px;
  min-height: 44px; max-height: 140px; overflow-y: auto;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 6px 10px; cursor: text;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
  box-sizing: border-box;
}
.token-input::-webkit-scrollbar { width: 4px; }
.token-input::-webkit-scrollbar-track { background: transparent; }
.token-input::-webkit-scrollbar-thumb { background: #c5c8d4; border-radius: 4px; }
.token-input::-webkit-scrollbar-thumb:hover { background: #a0a3b1; }
.token-input--focused { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.token-input--disabled { background: #f9fafb; opacity: .7; cursor: not-allowed; }

.token-chip {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 6px 4px 10px; border-radius: 20px;
  background: rgba(59,63,224,.1); color: #2a2e9e;
  font: 500 12px/1.4 'Inter', sans-serif;
  max-width: 200px; min-width: 0; flex-shrink: 0;
}
.token-chip-text {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0;
}
.token-chip--student { background: rgba(16,185,129,.1); color: #065f46; }
.token-group { opacity: .7; font-size: 11px; flex-shrink: 0; }
.token-remove {
  background: none; border: none; cursor: pointer; color: inherit;
  font-size: 15px; line-height: 1; padding: 0 2px; opacity: .55;
  transition: opacity .15s; flex-shrink: 0;
}
.token-remove:hover { opacity: 1; }

.token-field {
  flex: 1; min-width: 80px; border: none; outline: none;
  background: transparent; font: 13px/1 'Inter', sans-serif;
  color: var(--ink); padding: 3px 2px;
}
.token-field::placeholder { color: var(--ink-soft); }
.token-field:disabled { cursor: not-allowed; }

.picker-dropdown {
  position: absolute; top: calc(100% + 2px); left: 0; right: 0;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.14);
  z-index: 100; max-height: 220px; overflow-y: auto;
}
.picker-dropdown::-webkit-scrollbar { width: 4px; }
.picker-dropdown::-webkit-scrollbar-track { background: transparent; }
.picker-dropdown::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }
.picker-dropdown::-webkit-scrollbar-thumb:hover { background: #a0a3b1; }
.picker-option {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; text-align: left; background: none; border: none;
  padding: 9px 14px; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s; gap: 8px;
}
.picker-option:hover { background: rgba(59,63,224,.07); }
.picker-empty { padding: 10px 14px; font: 13px/1 'Inter', sans-serif; color: var(--ink-soft); }
.picker-hint  { padding: 6px 14px 10px; font: 11px/1.4 'Inter', sans-serif; color: #9ca3af; border-top: 1px solid var(--line); }

.suggest-group {
  font-size: 11px; color: var(--ink-soft);
  background: rgba(59,63,224,.08); border-radius: 10px; padding: 2px 8px; white-space: nowrap;
}

/* ── Group select row ── */
.group-select-row { display: flex; gap: 8px; }
.group-select-row .input { flex: 1; }
.btn-add-group {
  height: 38px; padding: 0 14px; flex-shrink: 0;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; font: 600 13px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; white-space: nowrap;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-add-group:hover:not(:disabled) { border-color: #10b981; color: #065f46; background: rgba(16,185,129,.07); }
.btn-add-group:disabled { opacity: .4; cursor: not-allowed; }

/* ── Hints ── */
.field-hint-warn { font-size: 12px; color: #d97706; margin: 4px 0 0; }
.submit-error    { font-size: 12px; color: #dc2626; margin: 0; }
.submit-success  { font-size: 12px; color: #059669; margin: 0; }

/* ── Submit button ── */
.btn-primary {
  align-self: center; padding: 0 32px; height: 40px; border: none;
  border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 14px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 8px 20px -8px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-primary:hover:not(:disabled) { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }
.btn-primary:disabled { opacity: .6; cursor: not-allowed; }

/* ── Responsive ── */
@media (max-width: 1280px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 960px) {
  .main { grid-template-columns: 1fr; }
  .col-right { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
}
@media (max-width: 600px) {
  .main { padding: 16px; gap: 16px; }
  .form-row { flex-direction: column; gap: 12px; }
  .stats-grid { grid-template-columns: 1fr 1fr; }
  .col-right { grid-template-columns: 1fr; }
}
</style>
