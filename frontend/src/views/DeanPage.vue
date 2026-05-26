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

const sidebarOpen = ref(false)

const upcomingRetakes = ref([])
const stats = ref([
  { label: 'Академических долгов', value: '—', accent: '#e63c5a' },
  { label: 'Назначено пересдач',   value: '—', accent: '#3b3fe0' },
  { label: 'Проводится сейчас',    value: '—', accent: '#f59e0b' },
  { label: 'Завершено в месяце',   value: '—', accent: '#10b981' },
])

onMounted(async () => {
  const [summaryRes, retakesRes, scheduledRes, disciplinesRes] = await Promise.allSettled([
    debtsApi.getSummary(),
    retakesApi.getAll({ limit: 200 }),
    retakesApi.getAll({ status: 'scheduled', limit: 10 }),
    disciplinesApi.getAll({ limit: 200 }),
  ])

  const discMap = {}
  if (disciplinesRes.status === 'fulfilled') {
    for (const d of disciplinesRes.value.data.items ?? [])
      discMap[d.id] = d.name || d.code
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
        subject: discMap[r.discipline_id] || 'Дисциплина',
        day: d.getDate(), month: d.getMonth() + 1, year: d.getFullYear(),
        time: `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`,
        building: r.building, room: r.room,
      }
    })
  }
})

// --- Форма назначения пересдачи ---
const retakeForm = reactive({
  subject:  '',
  type:     'normal',
  date:     null,
  time:     '',
  duration: 90,
  building: '',
  room:     '',
  teachers: [],
})

function submitRetake() {
  console.log('create retake', { ...retakeForm })
}

// --- Кастомный дропдаун типа пересдачи ---
const typeDropdownOpen = ref(false)
const typeOptions = [
  { value: 'normal',     label: 'Обычная' },
  { value: 'commission', label: 'С комиссией (мин. 3 преподавателя)' },
]
function selectType(val) {
  retakeForm.type = val
  typeDropdownOpen.value = false
}
function handleOutsideClick(e) {
  if (!e.target.closest('.custom-select')) typeDropdownOpen.value = false
}
onMounted(() => document.addEventListener('mousedown', handleOutsideClick))
onUnmounted(() => document.removeEventListener('mousedown', handleOutsideClick))

// --- Время ---
const timeHour   = ref(9)
const timeMinute = ref(0)
const hourDisplay   = ref('09')
const minuteDisplay = ref('00')

watch([timeHour, timeMinute], ([h, m]) => {
  retakeForm.time = `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}, { immediate: true })

function onTimeInput(e) {
  e.target.value = e.target.value.replace(/\D/g, '').slice(0, 2)
}
function onHourBlur() {
  const n = Math.max(0, Math.min(23, parseInt(hourDisplay.value, 10) || 0))
  timeHour.value = n
  hourDisplay.value = String(n).padStart(2, '0')
}
function onMinuteBlur() {
  const n = Math.max(0, Math.min(59, parseInt(minuteDisplay.value, 10) || 0))
  timeMinute.value = n
  minuteDisplay.value = String(n).padStart(2, '0')
}

// --- Длительность ---
const DURATION_STEP = 5
const DURATION_MIN  = 15
const DURATION_MAX  = 480
function decreaseDuration() { if (retakeForm.duration > DURATION_MIN) retakeForm.duration -= DURATION_STEP }
function increaseDuration()  { if (retakeForm.duration < DURATION_MAX) retakeForm.duration += DURATION_STEP }
function clampDuration() { retakeForm.duration = Math.max(DURATION_MIN, Math.min(DURATION_MAX, retakeForm.duration || DURATION_MIN)) }

// --- Преподаватели ---
const teacherInput   = ref('')
const isCommission   = computed(() => retakeForm.type === 'commission')
const minTeachers    = computed(() => isCommission.value ? 3 : 1)
const teacherCountOk = computed(() => retakeForm.teachers.length >= minTeachers.value)

function addTeacher() {
  const name = teacherInput.value.trim()
  if (!name) return
  if (!isCommission.value && retakeForm.teachers.length >= 1) return
  retakeForm.teachers.push(name)
  teacherInput.value = ''
}
function removeTeacher(i) { retakeForm.teachers.splice(i, 1) }

watch(() => retakeForm.type, (type) => {
  if (type === 'normal' && retakeForm.teachers.length > 1) retakeForm.teachers.splice(1)
})
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

              <div class="form-row">
                <div class="field">
                  <label>Дисциплина</label>
                  <input class="input" v-model="retakeForm.subject" placeholder="Название дисциплины" />
                </div>
                <div class="field">
                  <label>Тип пересдачи</label>
                  <div class="custom-select" :class="{ open: typeDropdownOpen }">
                    <button type="button" class="custom-select-trigger" @click="typeDropdownOpen = !typeDropdownOpen">
                      <span>{{ typeOptions.find(o => o.value === retakeForm.type)?.label }}</span>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                        <path d="M6 9l6 6 6-6"/>
                      </svg>
                    </button>
                    <div class="custom-select-dropdown">
                      <button
                        v-for="opt in typeOptions"
                        :key="opt.value"
                        type="button"
                        class="custom-select-option"
                        :class="{ selected: retakeForm.type === opt.value }"
                        @click="selectType(opt.value)"
                      >
                        {{ opt.label }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              <div class="form-row">
                <div class="field">
                  <label>Дата</label>
                  <VueDatePicker
                    v-model="retakeForm.date"
                    locale="ru"
                    format="dd.MM.yyyy"
                    model-type="format"
                    :enable-time-picker="false"
                    auto-apply
                    placeholder="дд.мм.гггг"
                  />
                </div>
                <div class="field field--shrink">
                  <label>Время</label>
                  <div class="time-picker">
                    <input
                      class="time-input"
                      type="text" inputmode="numeric"
                      v-model="hourDisplay"
                      maxlength="2" placeholder="00"
                      @input="onTimeInput"
                      @blur="onHourBlur"
                    />
                    <span class="time-colon">:</span>
                    <input
                      class="time-input"
                      type="text" inputmode="numeric"
                      v-model="minuteDisplay"
                      maxlength="2" placeholder="00"
                      @input="onTimeInput"
                      @blur="onMinuteBlur"
                    />
                  </div>
                </div>
                <div class="field">
                  <label>Длительность</label>
                  <div class="stepper">
                    <button type="button" class="stepper-btn" @click="decreaseDuration" :disabled="retakeForm.duration <= DURATION_MIN">−</button>
                    <input
                      class="stepper-input" type="number"
                      v-model.number="retakeForm.duration"
                      min="15" max="480"
                      @blur="clampDuration"
                    />
                    <button type="button" class="stepper-btn" @click="increaseDuration" :disabled="retakeForm.duration >= DURATION_MAX">+</button>
                  </div>
                </div>
              </div>

              <div class="form-row">
                <div class="field field--full">
                  <label>{{ isCommission ? 'Преподаватели' : 'Преподаватель' }}</label>
                  <div class="teacher-wrap">
                    <span v-for="(t, i) in retakeForm.teachers" :key="i" class="teacher-tag">
                      {{ t }}
                      <button type="button" class="teacher-tag-remove" @click="removeTeacher(i)">×</button>
                    </span>
                    <input
                      v-if="isCommission || retakeForm.teachers.length === 0"
                      class="teacher-input"
                      v-model="teacherInput"
                      :placeholder="isCommission ? 'ФИО преподавателя, затем Enter' : 'ФИО преподавателя'"
                      @keydown.enter.prevent="addTeacher"
                    />
                  </div>
                  <p v-if="isCommission && retakeForm.teachers.length > 0 && !teacherCountOk" class="field-hint-warn">
                    Для пересдачи с комиссией необходимо минимум 3 преподавателя
                  </p>
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

.page-dean {
  min-height: 100dvh;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

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
.section-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0 0 20px;
}

/* ── Retake form ── */
.retake-form { display: flex; flex-direction: column; gap: 16px; }
.form-row { display: flex; gap: 16px; }
.field { flex: 1; display: flex; flex-direction: column; gap: 6px; }
.field--shrink { flex: none; }
.field--full { width: 100%; }
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

/* ── Пикер времени ── */
.time-picker { display: flex; align-items: center; gap: 6px; }
.time-input {
  height: 38px; width: 64px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 8px;
  font: 14px/1 'Inter', sans-serif; color: var(--ink);
  text-align: center; outline: none; -moz-appearance: textfield;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.time-input::-webkit-outer-spin-button,
.time-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.time-input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.time-colon { font-size: 18px; font-weight: 700; color: var(--ink-soft); user-select: none; line-height: 1; }

/* ── Степпер длительности ── */
.stepper {
  display: flex; align-items: stretch; height: 38px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  overflow: hidden; background: #fff;
}
.stepper-btn {
  width: 38px; flex-shrink: 0; background: none; border: none;
  font-size: 20px; line-height: 1; color: var(--ink-soft); cursor: pointer;
  display: grid; place-items: center; transition: background .15s, color .15s;
}
.stepper-btn:hover:not(:disabled) { background: rgba(59,63,224,.07); color: var(--brand); }
.stepper-btn:disabled { opacity: .35; cursor: not-allowed; }
.stepper-input {
  flex: 1; border: none;
  border-left: 1px solid var(--line); border-right: 1px solid var(--line);
  background: transparent; outline: none;
  font: 13px/1 'Inter', sans-serif; font-weight: 500; color: var(--ink);
  text-align: center; -moz-appearance: textfield;
}
.stepper-input::-webkit-outer-spin-button,
.stepper-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }

/* ── Поле преподавателей ── */
.teacher-wrap {
  min-height: 40px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 4px 8px;
  display: flex; flex-wrap: wrap; gap: 6px; align-items: center;
  cursor: text; transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.teacher-wrap:focus-within {
  border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12);
}
.teacher-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 6px 4px 10px;
  background: rgba(59,63,224,.1); color: var(--brand-ink);
  border-radius: 20px; font-size: 12px; font-weight: 500;
}
.teacher-tag-remove {
  background: none; border: none; cursor: pointer;
  color: var(--brand-ink); font-size: 15px; line-height: 1;
  padding: 0 2px; opacity: .6; transition: opacity .15s;
}
.teacher-tag-remove:hover { opacity: 1; }
.teacher-input {
  border: none; outline: none; flex: 1; min-width: 180px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); background: transparent; padding: 4px 0;
}
.teacher-input::placeholder { color: #b7b9c2; }
.field-hint-warn { font-size: 12px; color: #d97706; margin: 4px 0 0; }

/* ── Кастомный select ── */
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
.custom-select-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.12);
  z-index: 50; overflow: hidden;
  opacity: 0; pointer-events: none; transform: translateY(-4px);
  transition: opacity .15s var(--ease), transform .15s var(--ease);
}
.custom-select.open .custom-select-dropdown {
  opacity: 1; pointer-events: all; transform: translateY(0);
}
.custom-select-option {
  display: block; width: 100%; text-align: left;
  background: none; border: none; padding: 10px 14px;
  font: 13px/1.4 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s;
}
.custom-select-option:hover { background: rgba(59,63,224,.06); }
.custom-select-option.selected { color: var(--brand); font-weight: 500; background: rgba(59,63,224,.05); }

.btn-primary {
  align-self: center; padding: 0 32px; height: 40px; border: none;
  border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 14px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 8px 20px -8px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-primary:hover { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }

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
