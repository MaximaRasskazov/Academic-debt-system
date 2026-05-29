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

// Реальные данные с бэка вместо моков.
const allDebts = ref([])
const allRetakes = ref([])
const disciplines = ref([])
const disciplineMap = ref({})
const dashLoading = ref(true)
const dashError = ref('')

async function loadDashboard() {
  dashLoading.value = true
  dashError.value = ''
  try {
    const [debtsResp, retakesResp, discsResp] = await Promise.all([
      debtsApi.listAll({ limit: 500 }),
      retakesApi.listAll({ limit: 200 }),
      disciplinesApi.list({ limit: 200 }),
    ])
    allDebts.value = debtsResp.items || debtsResp || []
    allRetakes.value = retakesResp.items || retakesResp || []
    disciplines.value = discsResp.items || discsResp || []
    disciplineMap.value = Object.fromEntries(
      disciplines.value.map((d) => [d.id, d.name]),
    )
  } catch (e) {
    dashError.value =
      e.response?.data?.message || 'Не удалось загрузить данные'
  } finally {
    dashLoading.value = false
  }
}

const stats = computed(() => {
  const now = new Date()
  const monthStart = new Date(now.getFullYear(), now.getMonth(), 1)
  return [
    {
      label: 'Академических долгов',
      value: allDebts.value.filter((d) => d.status === 'open').length,
      accent: '#e63c5a',
    },
    {
      label: 'Назначено пересдач',
      value: allRetakes.value.filter((r) => r.status === 'scheduled').length,
      accent: '#3b3fe0',
    },
    {
      label: 'Проводится сейчас',
      value: allRetakes.value.filter((r) => r.status === 'in_progress').length,
      accent: '#f59e0b',
    },
    {
      label: 'Завершено в месяце',
      value: allRetakes.value.filter(
        (r) =>
          r.status === 'completed' &&
          new Date(r.completed_at || r.scheduled_at) >= monthStart,
      ).length,
      accent: '#10b981',
    },
  ]
})

// Адаптируем retake → формат для UpcomingRetakes.
const upcomingRetakes = computed(() =>
  allRetakes.value
    .filter((r) => r.status === 'scheduled')
    .sort((a, b) => new Date(a.scheduled_at) - new Date(b.scheduled_at))
    .slice(0, 5)
    .map((r) => {
      const dt = new Date(r.scheduled_at)
      return {
        id: r.id,
        subject: disciplineMap.value[r.discipline_id] || 'Дисциплина',
        day: dt.getDate(),
        month: dt.getMonth() + 1,
        year: dt.getFullYear(),
        time: dt.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }),
        building: r.building,
        room: r.room,
      }
    }),
)

// --- Форма назначения пересдачи ---
// disciplineId/teachers хранят UUID, чтобы при submit не нужно было
// дополнительно искать по имени и не было риска отправить несуществующее.
const retakeForm = reactive({
  disciplineId: '',
  type:         'normal',
  date:         null,
  time:         '',
  duration:     90,
  building:     '',
  room:         '',
  teachers:     [], // массив объектов { id, fullName }
})

const submitting = ref(false)
const submitError = ref('')
const submitSuccess = ref('')

// Список преподавателей подтягиваем один раз при загрузке страницы —
// тогда multiselect работает без задержек на каждый клик.
const allTeachers = ref([])
async function loadTeachers() {
  try {
    const resp = await usersApi.list({ role: 'teacher', limit: 200 })
    allTeachers.value = (resp.items || []).map((u) => ({
      id: u.id,
      fullName: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' '),
      email: u.email,
    }))
  } catch (e) {
    // Не критично для рендера страницы — поле "преподаватели" просто
    // покажет пустой dropdown. В консоль уйдёт причина для отладки.
    console.warn('Не удалось загрузить список преподавателей', e)
  }
}

// Создание пересдачи: сначала POST /retakes, затем по одному запросу
// POST /retakes/:id/teachers для каждого выбранного. Студенты на этом
// шаге не добавляются — декан делает это отдельно из карточки пересдачи.
async function submitRetake() {
  submitError.value = ''
  submitSuccess.value = ''

  if (!retakeForm.disciplineId) {
    submitError.value = 'Выберите дисциплину'
    return
  }
  if (!retakeForm.date) {
    submitError.value = 'Укажите дату пересдачи'
    return
  }
  if (!retakeForm.building.trim() || !retakeForm.room.trim()) {
    submitError.value = 'Заполните корпус и аудиторию'
    return
  }
  if (!teacherCountOk.value) {
    submitError.value = `Нужно минимум ${minTeachers.value} преподаватель(ей)`
    return
  }

  // Собираем datetime: retakeForm.date + retakeForm.time
  const [hh, mm] = (retakeForm.time || '00:00').split(':').map(Number)
  const dt = new Date(retakeForm.date)
  dt.setHours(hh, mm, 0, 0)

  submitting.value = true
  try {
    const r = await retakesApi.create({
      discipline_id: retakeForm.disciplineId,
      kind: retakeForm.type === 'commission' ? 'commission' : 'regular',
      building: retakeForm.building.trim(),
      room: retakeForm.room.trim(),
      scheduled_at: dt.toISOString(),
      duration_minutes: retakeForm.duration,
    })

    // Привязываем выбранных преподавателей. Если один сбоит — продолжаем,
    // декан увидит итоговое предупреждение, но пересдача уже создана.
    const failed = []
    for (const t of retakeForm.teachers) {
      try {
        await retakesApi.addTeacher(r.id, t.id)
      } catch (err) {
        failed.push(t.fullName)
      }
    }

    if (failed.length > 0) {
      submitSuccess.value = `Пересдача создана, но не удалось добавить: ${failed.join(', ')}`
    } else {
      submitSuccess.value = 'Пересдача создана'
    }

    // Очищаем форму, перезагружаем список.
    retakeForm.disciplineId = ''
    retakeForm.building = ''
    retakeForm.room = ''
    retakeForm.teachers = []
    await loadDashboard()
  } catch (e) {
    submitError.value =
      e.response?.data?.message ||
      'Не удалось создать пересдачу. Проверьте поля'
  } finally {
    submitting.value = false
  }
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
  if (!e.target.closest('.discipline-select')) disciplineDropdownOpen.value = false
  if (!e.target.closest('.teacher-select')) teacherDropdownOpen.value = false
}

// --- Дисциплина: searchable select ---
const disciplineDropdownOpen = ref(false)
const disciplineSearch = ref('')
const selectedDiscipline = computed(() =>
  disciplines.value.find((d) => d.id === retakeForm.disciplineId),
)
const filteredDisciplines = computed(() => {
  const q = disciplineSearch.value.trim().toLowerCase()
  if (!q) return disciplines.value
  return disciplines.value.filter(
    (d) =>
      d.name.toLowerCase().includes(q) ||
      (d.code || '').toLowerCase().includes(q),
  )
})
function selectDiscipline(d) {
  retakeForm.disciplineId = d.id
  disciplineSearch.value = ''
  disciplineDropdownOpen.value = false
}

// --- Преподаватели: multiselect из загруженного списка ---
const teacherDropdownOpen = ref(false)
const teacherSearch = ref('')
const filteredTeachers = computed(() => {
  const q = teacherSearch.value.trim().toLowerCase()
  const selectedIds = new Set(retakeForm.teachers.map((t) => t.id))
  return allTeachers.value
    .filter((t) => !selectedIds.has(t.id))
    .filter((t) =>
      !q
        ? true
        : t.fullName.toLowerCase().includes(q) ||
          (t.email || '').toLowerCase().includes(q),
    )
})
function selectTeacher(t) {
  if (!isCommission.value && retakeForm.teachers.length >= 1) {
    // Для обычной пересдачи — заменяем единственного преподавателя.
    retakeForm.teachers = [t]
  } else {
    retakeForm.teachers.push(t)
  }
  teacherSearch.value = ''
  // Для commission удобно оставлять dropdown открытым, чтобы добавить
  // следующего без повторного клика.
  if (!isCommission.value) teacherDropdownOpen.value = false
}
function removeTeacherById(id) {
  retakeForm.teachers = retakeForm.teachers.filter((t) => t.id !== id)
}
onMounted(() => {
  document.addEventListener('mousedown', handleOutsideClick)
  loadDashboard()
  loadTeachers()
})
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
const isCommission   = computed(() => retakeForm.type === 'commission')
const minTeachers    = computed(() => isCommission.value ? 3 : 1)
const teacherCountOk = computed(() => retakeForm.teachers.length >= minTeachers.value)

// При смене типа пересдачи на "обычную" обрезаем список до одного — для
// regular достаточно одного преподавателя, остальные смутят бэк.
watch(() => retakeForm.type, (type) => {
  if (type === 'normal' && retakeForm.teachers.length > 1) {
    retakeForm.teachers = retakeForm.teachers.slice(0, 1)
  }
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

          <div v-if="dashLoading" class="dash-state">Загрузка…</div>
          <div v-else-if="dashError" class="dash-state dash-error">{{ dashError }}</div>
          <div v-else class="stats-grid">
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
                  <div class="custom-select discipline-select" :class="{ open: disciplineDropdownOpen }">
                    <button type="button" class="custom-select-trigger" @click="disciplineDropdownOpen = !disciplineDropdownOpen">
                      <span v-if="selectedDiscipline">{{ selectedDiscipline.name }}</span>
                      <span v-else class="placeholder">Выберите дисциплину</span>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                        <path d="M6 9l6 6 6-6"/>
                      </svg>
                    </button>
                    <div class="custom-select-dropdown dropdown-scroll">
                      <div class="dropdown-search">
                        <input
                          v-model="disciplineSearch"
                          class="dropdown-search-input"
                          placeholder="Поиск по названию или коду"
                          @click.stop
                        />
                      </div>
                      <button
                        v-for="d in filteredDisciplines"
                        :key="d.id"
                        type="button"
                        class="custom-select-option"
                        :class="{ selected: retakeForm.disciplineId === d.id }"
                        @click="selectDiscipline(d)"
                      >
                        <span class="opt-main">{{ d.name }}</span>
                        <span class="opt-sub">{{ d.code }}</span>
                      </button>
                      <div v-if="filteredDisciplines.length === 0" class="dropdown-empty">
                        Ничего не найдено
                      </div>
                    </div>
                  </div>
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
                  <div class="custom-select teacher-select" :class="{ open: teacherDropdownOpen }">
                    <div class="teacher-wrap" @click="teacherDropdownOpen = true">
                      <span v-for="t in retakeForm.teachers" :key="t.id" class="teacher-tag">
                        {{ t.fullName }}
                        <button type="button" class="teacher-tag-remove" @click.stop="removeTeacherById(t.id)">×</button>
                      </span>
                      <input
                        v-if="isCommission || retakeForm.teachers.length === 0"
                        class="teacher-input"
                        v-model="teacherSearch"
                        :placeholder="retakeForm.teachers.length === 0 ? 'Выберите преподавателя…' : 'Добавить ещё…'"
                        @focus="teacherDropdownOpen = true"
                      />
                    </div>
                    <div v-if="teacherDropdownOpen" class="custom-select-dropdown dropdown-scroll">
                      <button
                        v-for="t in filteredTeachers"
                        :key="t.id"
                        type="button"
                        class="custom-select-option"
                        @click="selectTeacher(t)"
                      >
                        <span class="opt-main">{{ t.fullName }}</span>
                        <span class="opt-sub">{{ t.email }}</span>
                      </button>
                      <div v-if="filteredTeachers.length === 0" class="dropdown-empty">
                        {{ allTeachers.length === 0 ? 'Список преподавателей не загружен' : 'Все подходящие уже выбраны' }}
                      </div>
                    </div>
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

              <p v-if="submitError" class="form-error">{{ submitError }}</p>
              <p v-if="submitSuccess" class="form-success">{{ submitSuccess }}</p>
              <button class="btn-primary" type="submit" :disabled="submitting">
                {{ submitting ? 'Создаём…' : 'Назначить пересдачу' }}
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

/* ── Двухстрочный вариант option (name + code/email) ── */
.opt-main { display: block; font: 500 13px/1.3 'Inter', sans-serif; color: var(--ink); }
.opt-sub  { display: block; font: 12px/1.3 'Inter', sans-serif; color: var(--ink-soft); margin-top: 2px; }

/* ── Прокручиваемый dropdown (для длинного списка) ── */
.dropdown-scroll { max-height: 260px; overflow-y: auto; }
.dropdown-search { padding: 8px; border-bottom: 1px solid var(--line); position: sticky; top: 0; background: #fff; z-index: 1; }
.dropdown-search-input {
  width: 100%; height: 32px; padding: 0 10px;
  border: 1.5px solid var(--line); border-radius: 6px;
  font: 12px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .15s, box-shadow .15s;
}
.dropdown-search-input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.dropdown-empty { padding: 14px; color: var(--ink-soft); text-align: center; font: 12px/1.4 'Inter', sans-serif; }
.placeholder { color: #b7b9c2; }

/* ── teacher-select: dropdown относительно теги-инпута ── */
.teacher-select { position: relative; }
.teacher-select .teacher-wrap { width: 100%; }
.teacher-select .custom-select-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.12);
  z-index: 50;
}

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
.dash-state {
  background: #fff; border-radius: 10px; padding: 24px;
  text-align: center; color: #6b7280; font-size: 14px;
  box-shadow: 0 2px 8px rgba(20,22,60,.07);
}
.dash-error { color: #d63a51; }
.form-error { color: #d63a51; font-size: 13px; margin: 8px 0 0; }
.form-success { color: #059669; font-size: 13px; margin: 8px 0 0; }
</style>
