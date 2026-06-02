<script setup>
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import VueDatePicker from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { disciplinesApi } from '../api/disciplines'
import { directoryApi } from '../api/directory'
import { retakeRequestsApi } from '../api/retakeRequests'
import { debtsApi } from '../api/debts'
import { useAuthStore } from '../stores/auth'
import { fmtDate as fmtDateTZ, fmtTime, toUtcISO } from '../utils/datetime'

const auth = useAuthStore()
const sidebarOpen = ref(false)

// ── Tabs ──────────────────────────────────────────────────
const activeTab = ref('my-requests')

// ── Shared constants ──────────────────────────────────────
const typeOptions = [
  { value: 'normal',     label: 'Обычная' },
  { value: 'commission', label: 'С комиссией (мин. 3 преподавателя)' },
]
const DURATION_STEP = 5, DURATION_MIN = 15, DURATION_MAX = 480
const STATUS_LABELS = { pending: 'Ожидает', approved: 'Одобрена', rejected: 'Отклонена' }
const TYPE_LABELS   = { normal: 'Обычная', regular: 'Обычная', commission: 'С комиссией' }

function clamp(val, min, max) { return Math.max(min, Math.min(max, val || min)) }

// ── Outside click (close dropdowns) ──────────────────────
// Закрывает все списки страницы, когда клик произошёл за пределами их
// контейнеров. Каждый дропдаун обёрнут в свой родитель, по его классу
// и определяем "снаружи или внутри".
//
// picker-wrap покрывает оба token-input'а: teacher и student.
// Они закрываются одним блоком, но это ок — если юзер кликнул мимо
// обоих, оба и должны закрыться.
function handleOutsideClick(e) {
  if (!e.target.closest('.custom-select')) {
    typeDropdownOpen.value = false
    discDropdownOpen.value = false
  }
  if (!e.target.closest('.group-custom-select')) groupDropdownOpen.value = false

  // picker-wrap содержит и input, и dropdown как siblings — клик внутри
  // не должен закрывать; клик снаружи — закрывает оба picker'а.
  if (!e.target.closest('.picker-wrap')) {
    teacherOpen.value = false
    studentOpen.value = false
  }
}
onMounted(async () => {
  document.addEventListener('mousedown', handleOutsideClick)
  // Старого префилла формы по route.query.retakeId больше нет: изменение
  // уже назначенной пересдачи теперь идёт через модалку «Запросить
  // изменения» на /retakes (POST /api/retake-change-requests), а не через
  // редирект на форму создания. См. коммит про этот flow.
  await Promise.allSettled([
    loadMyRequests(),
    // Преподаватель может запрашивать пересдачу только по СВОИМ дисциплинам.
    disciplinesApi.myAsTeacher().then(r => { disciplines.value = r.data.items ?? r.data ?? [] }).catch(() => {}),
  ])
})
onUnmounted(() => document.removeEventListener('mousedown', handleOutsideClick))

// ── My Requests data ──────────────────────────────────────
const myRequests    = ref([])
const myLoading     = ref(true)
const discMapMy     = ref({})

async function loadMyRequests() {
  myLoading.value = true
  try {
    const [reqRes, discsRes] = await Promise.allSettled([
      retakeRequestsApi.listMy({ limit: 200 }),
      disciplinesApi.getAll({ limit: 500 }),
    ])
    if (discsRes.status === 'fulfilled')
      (discsRes.value.data.items ?? []).forEach(d => { discMapMy.value[d.id] = d.name || d.code })
    if (reqRes.status === 'fulfilled')
      myRequests.value = reqRes.value.data.items ?? reqRes.value.data ?? []
  } finally {
    myLoading.value = false
  }
}

const statusFilter = ref('all')
const filteredRequests = computed(() =>
  statusFilter.value === 'all'
    ? myRequests.value
    : myRequests.value.filter(r => r.status === statusFilter.value)
)
const pendingCount = computed(() => myRequests.value.filter(r => r.status === 'pending').length)

// ── Submit form — discipline picker ───────────────────────
const disciplines       = ref([])
const disciplineId      = ref('')
const discSearch        = ref('')
const discDropdownOpen  = ref(false)

const selectedDiscipline = computed(() => disciplines.value.find(d => d.id === disciplineId.value))
const filteredDisciplines = computed(() => {
  const q = discSearch.value.toLowerCase()
  return q ? disciplines.value.filter(d => d.name.toLowerCase().includes(q) || (d.code||'').toLowerCase().includes(q)) : disciplines.value
})
function selectDiscipline(id) { disciplineId.value = id; discDropdownOpen.value = false; discSearch.value = '' }

// ── Submit form — participants ────────────────────────────
const availableStudents   = ref([])
const availableTeachers   = ref([])
const selectedStudents    = ref([])
const selectedTeachers    = ref([])
const loadingParticipants = ref(false)
const studentSearch       = ref('')
const teacherSearch       = ref('')
const studentOpen         = ref(false)
const teacherOpen         = ref(false)
const groupSelect         = ref('')
const groupDropdownOpen   = ref(false)

let teacherBlurTimer = null
let studentBlurTimer = null

function onTeacherFocus() {
  clearTimeout(teacherBlurTimer)
  studentOpen.value = false
  teacherOpen.value = true
}
function onTeacherBlur() {
  teacherBlurTimer = setTimeout(() => { teacherOpen.value = false }, 200)
}
function onStudentFocus() {
  clearTimeout(studentBlurTimer)
  teacherOpen.value = false
  studentOpen.value = true
}
function onStudentBlur() {
  studentBlurTimer = setTimeout(() => { studentOpen.value = false }, 200)
}

const selectedStudentIds = computed(() => new Set(selectedStudents.value.map(s => s.id)))
const selectedTeacherIds = computed(() => new Set(selectedTeachers.value.map(t => t.id)))
const availableGroups    = computed(() => [...new Set(availableStudents.value.map(s => s.group).filter(Boolean))])

const filteredStudents = computed(() => {
  const q = studentSearch.value.toLowerCase()
  return availableStudents.value
    .filter(s => !selectedStudentIds.value.has(s.id) && (s.name.toLowerCase().includes(q) || (s.group||'').toLowerCase().includes(q)))
    .slice(0, 8)
})
const filteredTeachers = computed(() => {
  const q = teacherSearch.value.toLowerCase()
  return availableTeachers.value
    .filter(t => !selectedTeacherIds.value.has(t.id) && t.name.toLowerCase().includes(q))
    .slice(0, 8)
})

watch(disciplineId, async (id) => {
  selectedStudents.value = []; selectedTeachers.value = []; groupSelect.value = ''
  availableStudents.value = []; availableTeachers.value = []
  if (!id) return
  loadingParticipants.value = true
  try {
    const debtorsRes = await directoryApi.listDebtors(id).catch(() => null)
    if (debtorsRes) {
      availableStudents.value = (debtorsRes.data.items ?? []).map((d) => ({
        id:     d.student_id,
        debtId: d.debt_id,
        name:   [d.last_name, d.first_name, d.middle_name].filter(Boolean).join(' ') || d.student_id,
        group:  d.group_name ?? '',
      }))
    }
    const teachersRes = await directoryApi.listTeachers().catch(() => null)
    if (teachersRes) {
      availableTeachers.value = (teachersRes.data.items ?? []).map((t) => ({
        id:   t.id,
        name: [t.last_name, t.first_name, t.middle_name].filter(Boolean).join(' '),
      }))
    }
    // Обычная пересдача → автоматически проставляем самого автора.
    applyTeacherDefault()
  } finally { loadingParticipants.value = false }
})

function addStudent(s) {
  clearTimeout(studentBlurTimer)
  if (!selectedStudentIds.value.has(s.id)) selectedStudents.value.push(s)
  studentSearch.value = ''
  studentOpen.value = true
}
function removeStudent(id) {
  selectedStudents.value = selectedStudents.value.filter(s => s.id !== id)
  studentOpen.value = true
}
function addGroup(g) {
  for (const s of availableStudents.value) {
    if (s.group === g && !selectedStudentIds.value.has(s.id)) selectedStudents.value.push(s)
  }
}
function selectGroup(g) { groupSelect.value = g; groupDropdownOpen.value = false }
function onStudentEnter() { if (filteredStudents.value.length) addStudent(filteredStudents.value[0]) }

function addTeacher(t) {
  clearTimeout(teacherBlurTimer)
  if (teacherLocked.value) return            // обычная пересдача — состав фиксирован
  if (selectedTeacherIds.value.has(t.id)) return
  selectedTeachers.value.push(t)
  teacherSearch.value = ''
  teacherOpen.value = true
}
function removeTeacher(id) {
  if (teacherLocked.value) return            // нельзя убрать самого автора в обычной
  selectedTeachers.value = selectedTeachers.value.filter(t => t.id !== id)
  teacherOpen.value = true
}
function onTeacherEnter() { if (filteredTeachers.value.length) addTeacher(filteredTeachers.value[0]) }

// ── Submit form — base fields ─────────────────────────────
const retakeType     = ref('normal')
const typeDropdownOpen = ref(false)
// Слоты дат: 1..3 вариантов «дата + часы + минуты». Преподаватель может
// предложить несколько — декан выберет один при одобрении. Первый слот
// обязателен, остальные опциональны.
const MAX_SLOTS = 3
const slots = ref([{ date: null, hour: '09', minute: '00' }])
function addSlot() {
  if (slots.value.length >= MAX_SLOTS) return
  slots.value.push({ date: null, hour: '09', minute: '00' })
}
function removeSlot(i) {
  if (slots.value.length <= 1) return
  slots.value.splice(i, 1)
}
const duration       = ref(90)
const building       = ref('')
const room           = ref('')
const description    = ref('')
const formError      = ref('')
const formSuccess    = ref(false)
const formSubmitting = ref(false)

const isCommission   = computed(() => retakeType.value === 'commission')
const minTeachers    = computed(() => isCommission.value ? 3 : 1)
const teacherCountOk = computed(() => selectedTeachers.value.length >= minTeachers.value)

// Текущий преподаватель как участник. Для обычной пересдачи он
// автоматически назначается единственным преподавателем (и не редактируется).
const selfTeacher = computed(() => {
  const u = auth.user
  if (!u?.id) return null
  return {
    id:   u.id,
    name: [u.lastName, u.firstName, u.middleName].filter(Boolean).join(' ') || u.email || 'Я',
  }
})
// Для обычной пересдачи выбор преподавателя заблокирован — это всегда сам автор.
const teacherLocked = computed(() => !isCommission.value)

// Приводит список преподавателей к правилу типа: обычная → только сам автор.
function applyTeacherDefault() {
  if (isCommission.value) return
  selectedTeachers.value = selfTeacher.value ? [selfTeacher.value] : []
}

function selectType(val) { retakeType.value = val; typeDropdownOpen.value = false }
function onHourBlur(slot)   { const n = clamp(parseInt(slot.hour,   10), 0, 23); slot.hour   = String(n).padStart(2, '0') }
function onMinuteBlur(slot) { const n = clamp(parseInt(slot.minute, 10), 0, 59); slot.minute = String(n).padStart(2, '0') }
function onTimeInput(e) { e.target.value = e.target.value.replace(/\D/g, '').slice(0, 2) }

// Форматтеры даты/времени для таблицы «Мои заявки». Без них шаблон
// вызывает _ctx.fmtDate is not a function и страница крашится в
// рендере списка. Используются также в RequestsPage.vue — точные копии.
// Дата/время в часовом поясе вуза (Ханты, UTC+5) — через utils/datetime,
// а не зону устройства. fmtTime импортирован напрямую.
function fmtDate(iso) {
  if (!iso) return '—'
  return fmtDateTZ(iso, { day: '2-digit', month: '2-digit', year: 'numeric' })
}

// Приводит дату из VueDatePicker ("ДД.ММ.ГГГГ") к "ГГГГ-ММ-ДД".
// Допускает и уже-ISO значение. Возвращает '' для некорректного ввода.
function toIsoDate(v) {
  if (!v) return ''
  if (/^\d{4}-\d{2}-\d{2}/.test(v)) return v.slice(0, 10)
  const m = /^(\d{2})\.(\d{2})\.(\d{4})$/.exec(v)
  return m ? `${m[3]}-${m[2]}-${m[1]}` : ''
}
function decreaseDuration() { if (duration.value > DURATION_MIN) duration.value -= DURATION_STEP }
function increaseDuration()  { if (duration.value < DURATION_MAX) duration.value += DURATION_STEP }
function clampDuration()     { duration.value = clamp(duration.value, DURATION_MIN, DURATION_MAX) }

// Смена типа: обычная → жёстко сам автор; комиссия → свободный выбор
// (начинаем с пустого списка, чтобы автор осознанно собрал состав ≥3).
watch(isCommission, (val) => {
  if (!val) {
    applyTeacherDefault()
  } else {
    // переход в комиссию: убираем авто-выбор, чтобы не путать с обычной
    selectedTeachers.value = []
  }
})

function resetForm() {
  disciplineId.value = ''; retakeType.value = 'normal'
  slots.value = [{ date: null, hour: '09', minute: '00' }]
  duration.value = 90; building.value = ''; room.value = ''; description.value = ''
  selectedStudents.value = []; selectedTeachers.value = []; groupSelect.value = ''
  formError.value = ''
}

const BUILDING_RE = /^[А-Яа-яA-Za-z0-9\s\-\/\.]{1,20}$/
const ROOM_RE     = /^[А-Яа-яA-Za-z0-9\s\-\/\.]{1,20}$/

async function submitForm() {
  formError.value = ''
  if (!disciplineId.value)  { formError.value = 'Выберите дисциплину'; return }

  // Собираем заполненные слоты в ISO. Первый слот обязателен; пустые
  // дополнительные слоты просто игнорируем.
  // VueDatePicker отдаёт дату в формате "ДД.ММ.ГГГГ" (model-type="format"),
  // toUtcISO ждёт "ГГГГ-ММ-ДД". Время вводится в зоне вуза (Ханты, UTC+5).
  const now = new Date()
  const proposedSlots = []
  for (let i = 0; i < slots.value.length; i++) {
    const sl = slots.value[i]
    if (!sl.date) {
      if (i === 0) { formError.value = 'Укажите дату'; return }
      continue // необязательный пустой слот
    }
    const isoDate = toIsoDate(sl.date)
    if (!isoDate) { formError.value = `Некорректная дата (вариант ${i + 1})`; return }
    const iso = toUtcISO(isoDate, `${sl.hour}:${sl.minute}`)
    if (new Date(iso) <= now) { formError.value = `Дата и время варианта ${i + 1} должны быть в будущем`; return }
    if (proposedSlots.includes(iso)) { formError.value = 'Варианты дат не должны повторяться'; return }
    proposedSlots.push(iso)
  }
  if (!proposedSlots.length) { formError.value = 'Укажите хотя бы одну дату'; return }
  const scheduledAtISO = proposedSlots[0]

  // Корпус и аудитория обязательны — иначе бэкенд отклоняет заявку (400),
  // и форма «молча» не отправлялась. Делаем валидацию явной на фронте.
  const bld = building.value.trim()
  const rm  = room.value.trim()
  if (!bld) { formError.value = 'Укажите корпус'; return }
  if (!BUILDING_RE.test(bld)) { formError.value = 'Некорректный номер корпуса (только буквы, цифры, до 20 символов)'; return }
  if (!rm)  { formError.value = 'Укажите аудиторию'; return }
  if (!ROOM_RE.test(rm))      { formError.value = 'Некорректный номер аудитории (только буквы, цифры, до 20 символов)'; return }

  if (!teacherCountOk.value) { formError.value = `Минимум ${minTeachers.value} преподавател${minTeachers.value > 1 ? 'я' : 'ь'}`; return }

  formSubmitting.value = true
  try {
    await retakeRequestsApi.submit({
      discipline_id:    disciplineId.value,
      kind:             isCommission.value ? 'commission' : 'regular',
      scheduled_at:     scheduledAtISO,
      // proposed_slots отправляем всегда (1..3); бэк продублирует первый
      // в scheduled_at и при >1 потребует выбора декана при одобрении.
      proposed_slots:   proposedSlots,
      duration_minutes: duration.value,
      building:         building.value,
      room:             room.value,
      notes:            description.value || undefined,
      teacher_ids:      selectedTeachers.value.map(t => t.id),
      student_debt_ids: selectedStudents.value.map(s => s.debtId),
    })
    resetForm()
    formSuccess.value = true
    setTimeout(() => { formSuccess.value = false }, 2500)
    activeTab.value = 'my-requests'
    await loadMyRequests()
  } catch (e) {
    formError.value = e.response?.data?.message || e.response?.data?.error || 'Ошибка при отправке заявки'
  } finally {
    formSubmitting.value = false
  }
}

// Edit-модалка удалена сознательно. По ТЗ редактировать заявку на
// СОЗДАНИЕ пересдачи нельзя — изменения УЖЕ НАЗНАЧЕННОЙ пересдачи
// идут через отдельный flow «заявка на изменение» (см.
// /api/retake-change-requests, кнопка «Запросить изменения» на
// /retakes). Pending-заявка живёт максимум до решения декана; если
// препод передумал — он может подать новую вместо неё.

// ── Detail modal ──────────────────────────────────────────
const detailModal = ref(null)
const studentsExpanded = ref(false)

async function openDetail(r) {
  const p = r.payload || {}
  const d = p.scheduled_at ? new Date(p.scheduled_at) : null
  const pad = (n) => String(n).padStart(2, '0')
  detailModal.value = {
    subject:      discMapMy.value[p.discipline_id] || 'Дисциплина',
    status:       r.status,
    submittedAt:  r.created_at
      ? new Date(r.created_at).toLocaleDateString('ru-RU', { day: '2-digit', month: 'long', year: 'numeric' })
      : '—',
    type:         p.kind || 'regular',
    date:         d ? d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' }) : '—',
    time:         d ? `${pad(d.getHours())}:${pad(d.getMinutes())}` : '—',
    duration:     p.duration_minutes ?? '—',
    building:     p.building || '',
    room:         p.room || '',
    group:        '',
    teachers:     [],
    students:     [],
    loadingParts: true,
    description:  p.notes || '',
    rejectReason: r.decision_reason || '',
    _raw:         r,
  }

  try {
    // Загружаем преподавателей и должников по дисциплине параллельно
    const [teachersRes, debtorsRes, ...debtResults] = await Promise.allSettled([
      directoryApi.listTeachers(),
      directoryApi.listDebtors(p.discipline_id),
      ...(p.student_debt_ids || []).map(dId => debtsApi.getById(dId)),
    ])

    if (!detailModal.value) return

    // Преподаватели: карта id → ФИО
    if (teachersRes.status === 'fulfilled') {
      const tList = teachersRes.value.data.items ?? teachersRes.value.data ?? []
      const tMap = {}
      tList.forEach(t => {
        tMap[t.id] = [t.last_name, t.first_name, t.middle_name].filter(Boolean).join(' ')
      })
      detailModal.value.teachers = (p.teacher_ids || [])
        .map(id => tMap[id])
        .filter(Boolean)
    }

    // Студенты: из listDebtors получаем ФИО по student_id
    // debtResults содержат {student_id} — резолвим через карту должников
    const studentMap = {}
    if (debtorsRes.status === 'fulfilled') {
      const dList = debtorsRes.value.data.items ?? debtorsRes.value.data ?? []
      dList.forEach(d => {
        studentMap[d.student_id] = [d.last_name, d.first_name, d.middle_name].filter(Boolean).join(' ')
      })
    }
    detailModal.value.students = debtResults
      .filter(r => r.status === 'fulfilled')
      .map(r => {
        const debt = r.value.data ?? r.value
        return studentMap[debt.student_id] || null
      })
      .filter(Boolean)
  } catch { /* не критично */ } finally {
    if (detailModal.value) detailModal.value.loadingParts = false
  }
}

function closeDetail() { detailModal.value = null; studentsExpanded.value = false }
</script>

<template>
  <div class="teacher-req-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-wrap">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">
        <div class="content-wrap">

          <!-- ── Tab switcher ── -->
          <div class="page-tabs">
            <button class="tab-btn" :class="{ active: activeTab === 'my-requests' }" @click="activeTab = 'my-requests'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 12h-6l-2 3H10l-2-3H2"/><path d="M5.45 5.11L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.45-6.89A2 2 0 0016.76 4H7.24a2 2 0 00-1.79 1.11z"/>
              </svg>
              Мои заявки
              <span v-if="pendingCount > 0" class="tab-badge">{{ pendingCount }}</span>
            </button>
            <button class="tab-btn" :class="{ active: activeTab === 'submit' }" @click="activeTab = 'submit'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="16"/><line x1="8" y1="12" x2="16" y2="12"/>
              </svg>
              Подать заявку
            </button>
          </div>

          <!-- ── My Requests tab ── -->
          <div v-if="activeTab === 'my-requests'" class="section-card">
            <div class="table-header">
              <h2 class="section-title" style="margin:0">Мои заявки</h2>
              <div class="table-header-actions">
                <select class="filter-select" v-model="statusFilter">
                  <option value="all">Все статусы</option>
                  <option value="pending">Ожидает</option>
                  <option value="approved">Одобрена</option>
                  <option value="rejected">Отклонена</option>
                </select>
                <button class="btn-primary btn-sm-primary" @click="activeTab = 'submit'">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                    <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
                  </svg>
                  Подать заявку
                </button>
              </div>
            </div>

            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr class="head-row">
                    <th>№</th>
                    <th>Дисциплина</th>
                    <th>Тип</th>
                    <th>Дата</th>
                    <th>Время</th>
                    <th>Статус</th>
                    <th>Подана</th>
                    <th>Действия</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in filteredRequests" :key="r.id">
                    <td class="td-num">{{ i + 1 }}</td>
                    <td class="td-subject">{{ discMapMy[r.payload?.discipline_id] || '—' }}</td>
                    <td>{{ TYPE_LABELS[r.payload?.kind] || r.payload?.kind }}</td>
                    <td class="td-nowrap">{{ fmtDate(r.payload?.scheduled_at) }}</td>
                    <td>{{ fmtTime(r.payload?.scheduled_at) || '—' }}</td>
                    <td>
                      <span class="status-badge" :class="'status-' + r.status">
                        {{ STATUS_LABELS[r.status] }}
                      </span>
                    </td>
                    <td class="td-nowrap td-soft">{{ fmtDate(r.created_at) }}</td>
                    <td>
                      <div class="action-btns">
                        <button class="btn-sm btn-outline" @click="openDetail(r)">Подробнее</button>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="filteredRequests.length === 0">
                    <td colspan="8" class="empty-row">
                      <div class="empty-cell">
                        <span>Заявок пока нет</span>
                        <button class="btn-link" @click="activeTab = 'submit'">Подать первую заявку →</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- ── Submit tab ── -->
          <div v-if="activeTab === 'submit'" class="section-card">
            <h2 class="section-title">Подать заявку на пересдачу</h2>

            <form class="retake-form" @submit.prevent="submitForm" novalidate>

              <div class="form-row">
                <div class="field" style="flex:2">
                  <label>Дисциплина</label>
                  <div class="custom-select" :class="{ open: discDropdownOpen }">
                    <button type="button" class="custom-select-trigger" @click="discDropdownOpen = !discDropdownOpen">
                      <span :class="{ placeholder: !disciplineId }">{{ selectedDiscipline?.name || 'Выберите дисциплину' }}</span>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 9l6 6 6-6"/></svg>
                    </button>
                    <div class="custom-select-dropdown">
                      <div class="dropdown-search-wrap">
                        <input class="dropdown-search" v-model="discSearch" placeholder="Поиск..." @click.stop />
                      </div>
                      <div class="dropdown-scroll">
                        <button v-for="d in filteredDisciplines" :key="d.id"
                          type="button" class="custom-select-option" :class="{ selected: disciplineId === d.id }"
                          @click="selectDiscipline(d.id)">
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
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 9l6 6 6-6"/></svg>
                    </button>
                    <div class="custom-select-dropdown">
                      <button v-for="opt in typeOptions" :key="opt.value" type="button" class="custom-select-option" :class="{ selected: retakeType === opt.value }" @click="selectType(opt.value)">
                        {{ opt.label }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Слоты дат: 1 обязательный + до 2 дополнительных вариантов -->
              <div v-for="(sl, i) in slots" :key="i" class="form-row">
                <div class="field" style="flex:2">
                  <label>{{ i === 0 ? 'Дата' : `Дата (вариант ${i + 1})` }}</label>
                  <VueDatePicker v-model="sl.date" locale="ru" format="dd.MM.yyyy" model-type="format" :enable-time-picker="false" auto-apply placeholder="дд.мм.гггг" />
                </div>
                <div class="field field--shrink">
                  <label>Время</label>
                  <div class="time-picker">
                    <input class="time-input" type="text" inputmode="numeric" v-model="sl.hour"   maxlength="2" placeholder="09" @input="onTimeInput" @blur="onHourBlur(sl)" />
                    <span class="time-colon">:</span>
                    <input class="time-input" type="text" inputmode="numeric" v-model="sl.minute" maxlength="2" placeholder="00" @input="onTimeInput" @blur="onMinuteBlur(sl)" />
                  </div>
                </div>
                <div v-if="i === 0" class="field">
                  <label>Длительность (мин)</label>
                  <div class="stepper">
                    <button type="button" class="stepper-btn" @click="decreaseDuration" :disabled="duration <= DURATION_MIN">−</button>
                    <input class="stepper-input" type="number" v-model.number="duration" min="15" max="480" @blur="clampDuration" />
                    <button type="button" class="stepper-btn" @click="increaseDuration"  :disabled="duration >= DURATION_MAX">+</button>
                  </div>
                </div>
                <div v-else class="field field--shrink slot-remove-field">
                  <label>&nbsp;</label>
                  <button type="button" class="btn-slot-remove" @click="removeSlot(i)" title="Удалить вариант">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg>
                  </button>
                </div>
              </div>

              <!-- Добавить ещё дату (до MAX_SLOTS вариантов) -->
              <div class="slot-add-row">
                <button type="button" class="btn-add-slot" :disabled="slots.length >= MAX_SLOTS" @click="addSlot">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                  Добавить дату
                </button>
                <span class="slot-hint">
                  {{ slots.length >= MAX_SLOTS ? `Максимум ${MAX_SLOTS} варианта` : 'Можно предложить несколько дат — декан выберет одну' }}
                </span>
              </div>

              <!-- Преподаватели -->
              <div class="form-row">
                <div class="field field--full picker-wrap">
                  <label>{{ isCommission ? 'Преподаватели (мин. 3)' : 'Преподаватель' }}</label>

                  <!-- Обычная пересдача: автор назначается автоматически, выбор заблокирован -->
                  <template v-if="teacherLocked">
                    <div class="token-input token-input--locked">
                      <span v-for="t in selectedTeachers" :key="t.id" class="token-chip">
                        <span class="token-chip-text">{{ t.name }}</span>
                      </span>
                      <span v-if="!selectedTeachers.length" class="locked-hint">Вы будете назначены преподавателем</span>
                    </div>
                    <p class="field-hint">Для обычной пересдачи преподавателем автоматически назначаетесь вы.</p>
                  </template>

                  <!-- Комиссия: свободный выбор состава -->
                  <template v-else>
                  <div
                    class="token-input"
                    :class="{ 'token-input--focused': teacherOpen, 'token-input--disabled': !disciplineId || loadingParticipants }"
                    @click="$event.currentTarget.querySelector('input')?.focus()"
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
                  </template>
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
                    @click="$event.currentTarget.querySelector('input')?.focus()"
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
                    <div class="group-custom-select" :class="{ open: groupDropdownOpen }">
                      <button
                        type="button" class="group-select-trigger"
                        :disabled="!availableGroups.length"
                        @click="groupDropdownOpen = !groupDropdownOpen"
                      >
                        <span :class="{ placeholder: !groupSelect }">
                          {{ groupSelect || (disciplineId ? (availableGroups.length ? 'Выберите группу' : 'Нет групп') : 'Сначала выберите дисциплину') }}
                        </span>
                        <svg class="select-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                          <polyline points="6 9 12 15 18 9"/>
                        </svg>
                      </button>
                      <div v-show="groupDropdownOpen && availableGroups.length" class="group-select-dropdown">
                        <button
                          v-for="g in availableGroups" :key="g"
                          type="button" class="group-select-option"
                          :class="{ selected: groupSelect === g }"
                          @click="selectGroup(g)"
                        >{{ g }}</button>
                      </div>
                    </div>
                    <button
                      type="button" class="btn-add-group"
                      :disabled="!groupSelect"
                      @click="addGroup(groupSelect); groupSelect = ''"
                    >+ Добавить</button>
                  </div>
                </div>
              </div>

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

              <div class="form-row">
                <div class="field field--full">
                  <label>Описание / причина</label>
                  <textarea class="input textarea" v-model="description" placeholder="Причина или дополнительные сведения…" rows="3" />
                </div>
              </div>

              <p v-if="formError"   class="form-msg form-error">{{ formError }}</p>
              <p v-if="formSuccess" class="form-msg form-success">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M20 6L9 17l-5-5"/></svg>
                Заявка успешно подана
              </p>

              <button class="btn-submit" type="submit" :disabled="formSubmitting">
                {{ formSubmitting ? 'Отправка…' : 'Отправить заявку' }}
              </button>

            </form>
          </div>

        </div>
      </main>
    </div>

    <!-- ── Detail modal ── -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="detailModal" class="modal-overlay" @click.self="closeDetail">
          <div class="modal modal--detail">

            <div class="modal-head">
              <span class="modal-title">{{ detailModal.subject }}</span>
              <button class="modal-close" @click="closeDetail" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
              </button>
            </div>

            <div class="modal-body">

              <!-- Status hero -->
              <div class="detail-hero" :class="'hero-' + detailModal.status">
                <div class="detail-hero-icon">
                  <svg v-if="detailModal.status === 'pending'"  viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  <svg v-if="detailModal.status === 'approved'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
                  <svg v-if="detailModal.status === 'rejected'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
                </div>
                <div>
                  <div class="detail-hero-status">{{ STATUS_LABELS[detailModal.status] }}</div>
                  <div class="detail-hero-date">Подана {{ detailModal.submittedAt }}</div>
                </div>
              </div>

              <!-- Reject reason -->
              <div v-if="detailModal.status === 'rejected' && detailModal.rejectReason" class="reject-banner">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
                <div><strong>Причина отказа:</strong> {{ detailModal.rejectReason }}</div>
              </div>

              <!-- Info grid -->
              <div class="detail-info-grid">
                <div class="detail-info-cell">
                  <span class="dil">Тип</span>
                  <span class="div">{{ TYPE_LABELS[detailModal.type] }}</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Дата и время</span>
                  <span class="div">{{ detailModal.date }} в {{ detailModal.time }}</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Длительность</span>
                  <span class="div">{{ detailModal.duration }} мин</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Группа</span>
                  <span class="div">{{ detailModal.group || '—' }}</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Корпус / Аудитория</span>
                  <span class="div">{{ detailModal.building || '—' }} / {{ detailModal.room || '—' }}</span>
                </div>
              </div>

              <!-- People -->
              <div v-if="detailModal.loadingParts" class="parts-loading">
                <div class="spinner-sm" /><span>Загрузка участников…</span>
              </div>
              <div v-else class="detail-people">
                <div class="detail-people-col">
                  <span class="dil">{{ detailModal.type === 'commission' ? 'Преподаватели' : 'Преподаватель' }}</span>
                  <div class="detail-tags">
                    <span v-for="t in detailModal.teachers" :key="t" class="tag">{{ t }}</span>
                    <span v-if="!detailModal.teachers.length" class="div">—</span>
                  </div>
                </div>
                <div class="detail-people-col">
                  <span class="dil">Студенты ({{ detailModal.students.length }})</span>
                  <div class="detail-tags">
                    <template v-if="detailModal.students.length">
                      <span
                        v-for="s in (studentsExpanded ? detailModal.students : detailModal.students.slice(0, 5))"
                        :key="s" class="tag tag--student"
                      >{{ s }}</span>
                      <button
                        v-if="detailModal.students.length > 5"
                        class="tag-more"
                        @click="studentsExpanded = !studentsExpanded"
                      >{{ studentsExpanded ? 'Скрыть' : `+ ещё ${detailModal.students.length - 5}` }}</button>
                    </template>
                    <span v-else class="div">—</span>
                  </div>
                </div>
              </div>

              <!-- Description -->
              <div v-if="detailModal.description" class="detail-description">
                <span class="dil">Описание</span>
                <p class="div">{{ detailModal.description }}</p>
              </div>

            </div>

            <div class="modal-foot">
              <button class="btn-outline" @click="closeDetail">Закрыть</button>
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

.teacher-req-root {
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

.main { flex: 1; padding: 24px; display: flex; justify-content: center; }

.content-wrap { width: 100%; display: flex; flex-direction: column; gap: 20px; }

/* ── Tab switcher ── */
.page-tabs {
  display: flex; gap: 6px;
  background: var(--card); border-radius: 14px; padding: 6px;
  box-shadow: var(--shadow);
}
.tab-btn {
  flex: 1; height: 44px; border: none; border-radius: 10px;
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink-soft); background: transparent;
  cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px;
  transition: background .2s var(--ease), color .2s var(--ease), box-shadow .2s var(--ease);
  position: relative;
}
.tab-btn svg { width: 16px; height: 16px; flex-shrink: 0; }
.tab-btn.active {
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff;
  box-shadow: 0 4px 14px -4px rgba(91,59,217,.5);
}
.tab-btn:not(.active):hover { background: rgba(59,63,224,.06); color: var(--brand); }
.tab-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 20px; padding: 0 5px; border-radius: 10px;
  background: rgba(245,158,11,.25); color: #b45309;
  font: 700 11px/1 'Inter', sans-serif;
}
.tab-btn.active .tab-badge { background: rgba(255,255,255,.25); color: #fff; }

/* ── Section card ── */
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px;
}
.section-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0 0 20px;
}

/* ── Table header ── */
.table-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 16px; gap: 12px; flex-wrap: wrap;
}
.filter-select {
  appearance: none; height: 36px; padding: 0 32px 0 12px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%236b7280' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E") no-repeat right 8px center;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer; outline: none;
  transition: border-color .2s;
}
.filter-select:focus { border-color: var(--brand); }

/* ── Table ── */
.table-wrap {
  overflow: auto; border: 1px solid var(--line); border-radius: var(--radius);
}
.data-table { width: 100%; border-collapse: collapse; font: 13px/1.4 'Inter', sans-serif; }
.head-row th {
  position: sticky; top: 0; z-index: 2;
  background: #f8f9fb; padding: 10px 14px;
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-align: left; white-space: nowrap; border-bottom: 1px solid var(--line);
}
.data-table tbody tr { transition: background .12s; }
.data-table tbody tr:hover { background: rgba(59,63,224,.03); }
.data-table td { padding: 11px 14px; border-bottom: 1px solid var(--line); color: var(--ink); vertical-align: middle; text-align: left; }
.data-table tbody tr:last-child td { border-bottom: none; }
.td-num     { color: var(--ink-soft); width: 40px; }
.td-subject { font-weight: 500; }
.td-nowrap  { white-space: nowrap; }
.td-soft    { color: var(--ink-soft); }
.empty-row  { text-align: center; color: var(--ink-soft); padding: 36px !important; }
.empty-cell { display: flex; flex-direction: column; align-items: center; gap: 8px; }
.btn-link   { background: none; border: none; cursor: pointer; color: var(--brand); font: 500 13px/1 'Inter', sans-serif; text-decoration: underline; padding: 0; }

.table-header-actions { display: flex; align-items: center; gap: 10px; }
.btn-sm-primary {
  display: inline-flex; align-items: center; gap: 6px;
  height: 36px !important; padding: 0 16px !important; font-size: 13px !important;
}
.btn-sm-primary svg { width: 14px; height: 14px; }

/* ── Action buttons ── */
.action-btns { display: flex; gap: 6px; }
.btn-sm { height: 30px !important; padding: 0 12px !important; font-size: 12px !important; }

/* ── Status badges ── */
.status-badge {
  display: inline-block; padding: 3px 10px; border-radius: 20px;
  font: 600 11px/1.4 'Inter', sans-serif; white-space: nowrap;
}
.status-pending  { background: rgba(245,158,11,.12); color: #b45309; }
.status-approved { background: rgba(16,185,129,.12);  color: #065f46; }
.status-rejected { background: rgba(220,38,38,.10);   color: #b91c1c; }

/* ── Form ── */
.retake-form { display: flex; flex-direction: column; gap: 16px; }
.form-row { display: flex; gap: 16px; }
.field { flex: 1; display: flex; flex-direction: column; gap: 6px; }
.field--shrink { flex: none; }
.field--full   { width: 100%; }
.field label   { font-size: 13px; font-weight: 500; color: var(--ink); text-align: left; }

.input {
  appearance: none; height: 38px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.input::placeholder { color: #b7b9c2; }
.textarea { height: auto; padding: 10px 12px; resize: vertical; line-height: 1.5; }

/* ── Time picker ── */
.time-picker { display: flex; align-items: center; gap: 6px; }
.time-input {
  height: 38px; width: 64px; text-align: center;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 8px;
  font: 14px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
  -moz-appearance: textfield;
}
.time-input::-webkit-outer-spin-button,
.time-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.time-input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.time-colon { font-size: 18px; font-weight: 700; color: var(--ink-soft); user-select: none; }

/* ── Duration stepper ── */
.stepper { display: flex; align-items: stretch; height: 38px; border: 1.5px solid var(--line); border-radius: var(--radius); overflow: hidden; background: #fff; }
.stepper-btn { width: 38px; flex-shrink: 0; background: none; border: none; font-size: 20px; color: var(--ink-soft); cursor: pointer; display: grid; place-items: center; transition: background .15s, color .15s; }
.stepper-btn:hover:not(:disabled) { background: rgba(59,63,224,.07); color: var(--brand); }
.stepper-btn:disabled { opacity: .35; cursor: not-allowed; }
.stepper-input { flex: 1; border: none; border-left: 1px solid var(--line); border-right: 1px solid var(--line); background: transparent; outline: none; font: 500 13px/1 'Inter', sans-serif; color: var(--ink); text-align: center; -moz-appearance: textfield; }
.stepper-input::-webkit-outer-spin-button,
.stepper-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }

/* ── Date slots ── */
.slot-remove-field { justify-content: flex-start; }
.btn-slot-remove {
  height: 38px; width: 44px; flex-shrink: 0;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; color: var(--ink-soft); cursor: pointer;
  display: grid; place-items: center;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-slot-remove:hover { border-color: #dc2626; color: #dc2626; background: rgba(220,38,38,.05); }
.btn-slot-remove svg { width: 16px; height: 16px; }
.slot-add-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-top: -4px; }
.btn-add-slot {
  display: inline-flex; align-items: center; gap: 6px;
  height: 34px; padding: 0 14px; border-radius: var(--radius);
  border: 1.5px dashed var(--line); background: #fff; color: var(--brand);
  font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  transition: border-color .15s, background .15s, opacity .15s;
}
.btn-add-slot:hover:not(:disabled) { border-color: var(--brand); background: rgba(59,63,224,.05); }
.btn-add-slot:disabled { opacity: .45; cursor: not-allowed; color: var(--ink-soft); }
.btn-add-slot svg { width: 14px; height: 14px; }
.slot-hint { font: 12px/1.4 'Inter', sans-serif; color: var(--ink-soft); }

/* ── Tags ── */
.tags-wrap {
  min-height: 40px; border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 4px 8px;
  display: flex; flex-wrap: wrap; gap: 6px; align-items: center;
  cursor: text; transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.tags-wrap:focus-within { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.tag {
  display: inline-flex; align-items: center; gap: 4px; padding: 4px 6px 4px 10px;
  background: rgba(59,63,224,.1); color: var(--brand-ink); border-radius: 20px; font-size: 12px; font-weight: 500;
}
.tag--student { background: rgba(16,185,129,.1); color: #065f46; }
.tag-remove { background: none; border: none; cursor: pointer; color: inherit; font-size: 15px; line-height: 1; padding: 0 2px; opacity: .6; transition: opacity .15s; }
.tag-remove:hover { opacity: 1; }
.tag-input { border: none; outline: none; flex: 1; min-width: 180px; font: 13px/1 'Inter', sans-serif; color: var(--ink); background: transparent; padding: 4px 0; }
.tag-input::placeholder { color: #b7b9c2; }
.hint-warn { font-size: 12px; color: #d97706; margin: 4px 0 0; }
.field-hint-warn { font: 12px/1.4 'Inter', sans-serif; color: #d97706; margin: 4px 0 0; }

/* ── Token input (как у декана) ── */
.picker-wrap { position: relative; }
.picker-wrap-field { position: relative; }
.token-input {
  display: flex; flex-wrap: wrap; align-items: center; gap: 6px;
  min-height: 44px; max-height: 140px; overflow-y: auto;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 6px 10px; cursor: text;
  transition: border-color .2s, box-shadow .2s; box-sizing: border-box;
}
.token-input::-webkit-scrollbar { width: 4px; }
.token-input::-webkit-scrollbar-thumb { background: #c5c8d4; border-radius: 4px; }
.token-input--focused { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.token-input--disabled { background: #f9fafb; opacity: .7; cursor: not-allowed; }
.token-input--locked { background: #f9fafb; cursor: default; }
.locked-hint { font: 13px/1 'Inter', sans-serif; color: var(--ink-soft); padding: 3px 2px; }
.field-hint { font: 12px/1.4 'Inter', sans-serif; color: var(--ink-soft); margin: 4px 0 0; }
.token-chip {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 6px 4px 10px; border-radius: 20px;
  background: rgba(59,63,224,.1); color: #2a2e9e;
  font: 500 12px/1.4 'Inter', sans-serif; max-width: 200px; flex-shrink: 0;
}
.token-chip--student { background: rgba(16,185,129,.1); color: #065f46; }
.token-chip-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.token-group { opacity: .7; font-size: 11px; flex-shrink: 0; }
.token-remove { background: none; border: none; cursor: pointer; color: inherit; font-size: 15px; line-height: 1; padding: 0 2px; opacity: .55; flex-shrink: 0; }
.token-remove:hover { opacity: 1; }
.token-field { flex: 1; min-width: 80px; border: none; outline: none; background: transparent; font: 13px/1 'Inter', sans-serif; color: var(--ink); padding: 3px 2px; }
.token-field::placeholder { color: var(--ink-soft); }
.token-field:disabled { cursor: not-allowed; }

.picker-dropdown {
  position: absolute; top: calc(100% + 2px); left: 0; right: 0; z-index: 100;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.14); max-height: 220px; overflow-y: auto;
}
.picker-dropdown::-webkit-scrollbar { width: 4px; }
.picker-dropdown::-webkit-scrollbar-thumb { background: #c5c8d4; border-radius: 4px; }
.picker-option {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; text-align: left; background: none; border: none;
  padding: 9px 14px; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s; gap: 8px;
}
.picker-option:hover { background: rgba(59,63,224,.07); }
.picker-empty { padding: 10px 14px; font: 13px/1 'Inter', sans-serif; color: var(--ink-soft); }
.suggest-group { font-size: 11px; color: var(--ink-soft); background: rgba(59,63,224,.08); border-radius: 10px; padding: 2px 8px; }

/* Поиск + скролл в дропдауне дисциплин — идентично странице декана */
.dropdown-search-wrap { padding: 8px 8px 4px; border-bottom: 1px solid var(--line); }
.dropdown-search {
  width: 100%; height: 30px; border: 1.5px solid var(--line); border-radius: 6px;
  padding: 0 10px; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  outline: none; background: #fff;
}
.dropdown-search:focus { border-color: var(--brand); }
.dropdown-scroll { max-height: 240px; overflow-y: auto; }
.dropdown-scroll::-webkit-scrollbar { width: 4px; }
.dropdown-scroll::-webkit-scrollbar-track { background: transparent; }
.dropdown-scroll::-webkit-scrollbar-thumb { background: #c5c8d4; border-radius: 4px; }
.dropdown-scroll::-webkit-scrollbar-thumb:hover { background: #a0a3b1; }
.dropdown-empty { padding: 10px 14px; font: 13px/1 'Inter', sans-serif; color: var(--ink-soft); }
.disc-name { flex: 1; }
.disc-code { font-size: 11px; color: var(--ink-soft); white-space: nowrap; }

/* ── Group custom select ── */
.group-select-row { display: flex; gap: 8px; }
.group-custom-select { position: relative; flex: 1; }
.group-select-trigger {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; height: 40px; padding: 0 12px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: border-color .2s;
}
.group-select-trigger:disabled { opacity: .5; cursor: not-allowed; background: #f9fafb; }
.group-custom-select.open .group-select-trigger { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.group-select-trigger .placeholder { color: var(--ink-soft); }
.select-arrow { width: 14px; height: 14px; color: var(--ink-soft); flex-shrink: 0; transition: transform .2s; }
.group-custom-select.open .select-arrow { transform: rotate(180deg); }
.group-select-dropdown {
  position: absolute; top: calc(100% + 2px); left: 0; right: 0; z-index: 100;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.14); max-height: 200px; overflow-y: auto;
}
.group-select-option {
  display: block; width: 100%; text-align: left; padding: 9px 14px;
  background: none; border: none; font: 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer;
}
.group-select-option:hover { background: rgba(59,63,224,.07); }
.group-select-option.selected { color: var(--brand); font-weight: 600; }

/* ── Custom select ── */
.custom-select { position: relative; }
.custom-select-trigger {
  appearance: none; width: 100%; height: 38px; text-align: left;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 36px 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink);
  display: flex; align-items: center; justify-content: space-between;
  cursor: pointer; transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.custom-select-trigger:focus,
.custom-select.open .custom-select-trigger { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); outline: none; }
.custom-select-trigger svg { width: 16px; height: 16px; flex-shrink: 0; color: var(--ink-soft); transition: transform .2s var(--ease); position: absolute; right: 10px; }
.custom-select.open .custom-select-trigger svg { transform: rotate(180deg); }
.custom-select-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.12); z-index: 50; overflow: hidden;
  opacity: 0; pointer-events: none; transform: translateY(-4px);
  transition: opacity .15s var(--ease), transform .15s var(--ease);
}
.custom-select.open .custom-select-dropdown { opacity: 1; pointer-events: all; transform: translateY(0); }
.custom-select-option {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  width: 100%; text-align: left; background: none; border: none;
  padding: 9px 14px; font: 13px/1.4 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s;
}
.custom-select-option:hover { background: rgba(59,63,224,.06); }
.custom-select-option.selected { color: var(--brand); font-weight: 500; background: rgba(59,63,224,.05); }

/* ── Form messages & submit ── */
.form-msg { font: 13px/1.4 'Inter', sans-serif; margin: 0; display: flex; align-items: center; gap: 6px; }
.form-error   { color: #dc2626; }
.form-success { color: #059669; }

/* ── Real API submit form extras ── */
.loading-hint { font: 13px/1.5 'Inter', sans-serif; color: var(--ink-soft); padding: 10px 0; }

.current-values {
  background: var(--bg); border-radius: 10px; padding: 14px 16px;
  display: flex; flex-direction: column; gap: 8px;
}
.cv-label { font: 600 11px/1 'Inter', sans-serif; color: var(--ink-soft); text-transform: uppercase; letter-spacing: .05em; }
.cv-grid  { display: flex; flex-wrap: wrap; gap: 12px 24px; }
.cv-item  { display: flex; flex-direction: column; gap: 3px; }
.cv-key   { font: 500 11px/1 'Inter', sans-serif; color: var(--ink-soft); }
.cv-val   { font: 600 13px/1 'Inter', sans-serif; color: var(--ink); }

.changes-header {
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-transform: uppercase; letter-spacing: .05em;
  padding-bottom: 4px; border-bottom: 1px solid var(--line);
}
.form-success svg { width: 15px; height: 15px; }

.btn-submit {
  align-self: flex-start; padding: 0 32px; height: 42px; border: none;
  border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 14px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 8px 20px -8px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-submit:hover { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }

/* ── Buttons ── */
.btn-primary {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 0 18px; height: 40px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 6px 18px -6px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-primary:hover { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }
.btn-primary svg { width: 15px; height: 15px; }

.btn-outline {
  padding: 0 18px; height: 40px;
  background: #fff; border: 1.5px solid #c5c7d4; border-radius: var(--radius);
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer;
  box-shadow: 0 1px 4px rgba(20,22,60,.08);
  transition: border-color .15s, color .15s, box-shadow .15s;
}
.btn-outline:hover { border-color: var(--brand); color: var(--brand); box-shadow: 0 2px 10px rgba(59,63,224,.14); }

/* ── Modal overlay ── */
.modal-overlay {
  position: fixed; inset: 0; z-index: 400;
  background: rgba(10,12,30,.5);
  backdrop-filter: blur(3px); -webkit-backdrop-filter: blur(3px);
  display: flex; align-items: center; justify-content: center;
  padding: 20px;
  /* CSS-переменные для teleport-контента (выходит за пределы .teacher-req-root) */
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
  font-family: 'Inter', system-ui, sans-serif;
}

/* ── Modal base ── */
.modal {
  background: var(--card); border-radius: 16px;
  box-shadow: 0 24px 64px -12px rgba(10,12,30,.3);
  width: 100%; display: flex; flex-direction: column; overflow: hidden;
  max-height: 90vh;
}
.modal--detail { max-width: 520px; }
.modal--edit   { max-width: 720px; }

.modal-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 20px; border-bottom: 1px solid var(--line); flex-shrink: 0;
}
.modal-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 15px; font-weight: 700; color: #3C38B6;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: calc(100% - 48px);
}
.modal-close {
  width: 32px; height: 32px; border-radius: 8px; flex-shrink: 0;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s, color .15s;
}
.modal-close:hover { background: var(--bg); color: var(--ink); }
.modal-close svg { width: 16px; height: 16px; }

.modal-body { flex: 1; overflow-y: auto; padding: 20px; }
.modal-body::-webkit-scrollbar { width: 4px; }
.modal-body::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }
.modal-body--form { padding: 20px 20px 4px; }

.modal-foot {
  display: flex; justify-content: flex-end; gap: 10px;
  padding: 14px 20px; border-top: 1px solid var(--line); flex-shrink: 0;
}

/* ── Detail modal content ── */
.detail-hero {
  display: flex; align-items: center; gap: 14px;
  padding: 14px 16px; border-radius: 12px; margin-bottom: 18px;
}
.hero-pending  { background: rgba(245,158,11,.08); border: 1px solid rgba(245,158,11,.2); }
.hero-approved { background: rgba(16,185,129,.08); border: 1px solid rgba(16,185,129,.2); }
.hero-rejected { background: rgba(220,38,38,.06);  border: 1px solid rgba(220,38,38,.15); }

.detail-hero-icon {
  width: 42px; height: 42px; border-radius: 50%;
  display: grid; place-items: center; flex-shrink: 0;
}
.hero-pending  .detail-hero-icon { background: rgba(245,158,11,.15); color: #b45309; }
.hero-approved .detail-hero-icon { background: rgba(16,185,129,.18);  color: #065f46; }
.hero-rejected .detail-hero-icon { background: rgba(220,38,38,.12);   color: #b91c1c; }
.detail-hero-icon svg { width: 20px; height: 20px; }

.detail-hero-status { font: 700 14px/1 'Inter', sans-serif; color: var(--ink); margin-bottom: 4px; }
.detail-hero-date   { font: 12px/1 'Inter', sans-serif; color: var(--ink-soft); }

.reject-banner {
  display: flex; align-items: flex-start; gap: 10px;
  background: rgba(220,38,38,.06); border: 1px solid rgba(220,38,38,.2);
  border-radius: 10px; padding: 12px 14px;
  font: 13px/1.5 'Inter', sans-serif; color: #b91c1c; margin-bottom: 16px;
}
.reject-banner svg { width: 16px; height: 16px; flex-shrink: 0; margin-top: 1px; }

.detail-info-grid {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 0; border: 1px solid var(--line); border-radius: 10px; overflow: hidden;
  margin-bottom: 16px;
}
.detail-info-cell {
  display: flex; flex-direction: column; gap: 4px;
  padding: 12px 14px; border-bottom: 1px solid var(--line);
  border-right: 1px solid var(--line);
}
.detail-info-cell:nth-child(2n) { border-right: none; }
.detail-info-cell:nth-last-child(-n+2) { border-bottom: none; }

.detail-people {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 12px; margin-bottom: 16px;
}
.detail-people-col { display: flex; flex-direction: column; gap: 8px; }
.detail-tags { display: flex; flex-wrap: wrap; gap: 6px; }

.detail-description {
  display: flex; flex-direction: column; gap: 6px;
  padding: 12px 14px; background: var(--bg); border-radius: 10px;
}
.detail-description .div { margin: 0; white-space: pre-wrap; }

.dil {
  font: 600 11px/1 'Inter', sans-serif;
  color: var(--ink-soft); text-transform: uppercase; letter-spacing: .05em;
}
.div { font: 13px/1.5 'Inter', sans-serif; color: var(--ink); }

/* ── Transition ── */
.modal-enter-active { transition: opacity .2s var(--ease); }
.modal-leave-active { transition: opacity .18s var(--ease); }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .modal { transition: transform .25s var(--ease); }
.modal-leave-active  .modal { transition: transform .2s  var(--ease); }
.modal-enter-from .modal, .modal-leave-to .modal { transform: scale(.96) translateY(12px); }

/* ── Participants loading / expand ── */
.parts-loading {
  display: flex; align-items: center; gap: 8px;
  font: 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  padding: 4px 0; margin-bottom: 16px;
}
.spinner-sm {
  width: 16px; height: 16px; border-radius: 50%;
  border: 2px solid var(--line); border-top-color: var(--brand);
  animation: spin .8s linear infinite; flex-shrink: 0;
}
.tag-more {
  display: inline-flex; align-items: center; padding: 4px 10px;
  background: none; border: 1.5px dashed var(--line); border-radius: 20px;
  color: var(--ink-soft); font: 500 12px/1.4 'Inter', sans-serif;
  cursor: pointer; transition: border-color .15s, color .15s;
}
.tag-more:hover { border-color: var(--brand); color: var(--brand); }

.btn-add-group {
  height: 38px; padding: 0 14px; flex-shrink: 0;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; font: 600 13px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; white-space: nowrap;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-add-group:hover { border-color: #10b981; color: #065f46; background: rgba(16,185,129,.07); }

/* ── Responsive ── */
@media (max-width: 640px) {
  .main { padding: 16px; }
  .form-row { flex-direction: column; gap: 12px; }
  .detail-info-grid { grid-template-columns: 1fr; }
  .detail-info-cell { border-right: none !important; }
  .detail-info-cell:last-child { border-bottom: none; }
  .detail-people { grid-template-columns: 1fr; }
}
</style>
