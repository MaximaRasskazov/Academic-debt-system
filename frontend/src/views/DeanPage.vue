<script setup>
import { ref, computed, reactive, watch, onMounted, onUnmounted } from 'vue'
import VueDatePicker from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'

import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'
import StatCard from '../components/StatCard.vue'
import EmptyPlate from '../components/EmptyPlate.vue'
import FilterSelect from '../components/FilterSelect.vue'
import { debtsApi } from '../api/debts'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { usersApi } from '../api/users'
import { directoryApi } from '../api/directory'
import { toUtcISO, partsInTZ } from '../utils/datetime'

const sidebarOpen = ref(false)

// ── Stats ─────────────────────────────────────────────────────
const upcomingRetakes = ref([])
const stats = ref([
  { key: 'debts',     label: 'Академические долги', value: '—', accent: '#e63c5a' },
  { key: 'scheduled', label: 'Запланировано',       value: '—', accent: '#3b3fe0' },
  { key: 'progress',  label: 'Проводится сейчас',   value: '—', accent: '#f59e0b' },
  { key: 'done',      label: 'Завершено в месяце',  value: '—', accent: '#10b981' },
])

// Какая карточка раскрыта: 'debts' | 'scheduled' | null
const activePanel = ref(null)
function togglePanel(key) {
  if (key !== 'debts' && key !== 'scheduled') return
  activePanel.value = activePanel.value === key ? null : key
  if (activePanel.value === 'debts' && debtTableRows.value.length === 0) loadDebtTable()
}

// ── Debt summary table (панель декана) ────────────────────────
const debtTableLoading = ref(false)
// Только актуальные долги (open). Закрытые/отменённые не показываем.
const debtTableRows    = ref([])   // [{id, discipline, disciplineCode, student, group, teacher}]

// Фильтры: группа / преподаватель / предмет (выпадающие списки)
const filterGroup   = ref('')
const filterTeacher = ref('')
const filterDisc    = ref('')

const debtGroups   = computed(() => [...new Set(debtTableRows.value.map(r => r.group).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru')))
const debtTeachers = computed(() => [...new Set(debtTableRows.value.map(r => r.teacher).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru')))
const debtDiscs    = computed(() => [...new Set(debtTableRows.value.map(r => r.discipline).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru')))

const hasActiveFilters = computed(() => !!(filterGroup.value || filterTeacher.value || filterDisc.value))
function resetFilters() {
  filterGroup.value = ''
  filterTeacher.value = ''
  filterDisc.value = ''
}

const filteredDebtRows = computed(() =>
  debtTableRows.value.filter(r =>
    (!filterGroup.value   || r.group === filterGroup.value) &&
    (!filterTeacher.value || r.teacher === filterTeacher.value) &&
    (!filterDisc.value    || r.discipline === filterDisc.value)
  )
)

// Тянет все страницы списочного эндпойнта (limit/offset → {items,total}).
// Нужно, чтобы таблица и выгрузка содержали ВСЕ записи, а не первую страницу.
async function fetchAllPages(apiFn, extraParams = {}, pageSize = 500) {
  const all = []
  let offset = 0
  for (;;) {
    const res = await apiFn({ ...extraParams, limit: pageSize, offset })
    const items = res.data.items ?? res.data ?? []
    all.push(...items)
    const total = res.data.total
    if (items.length < pageSize || (typeof total === 'number' && all.length >= total)) break
    offset += pageSize
  }
  return all
}

async function loadDebtTable() {
  debtTableLoading.value = true
  try {
    const [debtsRes, usersRes, discsRes] = await Promise.allSettled([
      fetchAllPages(debtsApi.getAll),
      fetchAllPages(usersApi.getAll, {}, 1000),
      fetchAllPages(disciplinesApi.getAll),
    ])

    const userMap = {}
    if (usersRes.status === 'fulfilled') {
      for (const u of usersRes.value) {
        userMap[u.id] = {
          name: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ') || u.email,
          group: u.group_name || '',
        }
      }
    }

    const discMapLocal = {}
    if (discsRes.status === 'fulfilled') {
      for (const d of discsRes.value) {
        discMapLocal[d.id] = { name: d.name, code: d.code }
      }
    }

    const rows = []
    if (debtsRes.status === 'fulfilled') {
      for (const d of debtsRes.value) {
        if (d.deleted_at) continue
        if (d.status !== 'open') continue   // только актуальные долги
        const disc    = discMapLocal[d.discipline_id] ?? { name: d.discipline_id, code: '' }
        const student = userMap[d.student_id] ?? { name: d.student_id, group: '' }
        const teacher = userMap[d.issued_by]  ?? { name: d.issued_by, group: '' }
        rows.push({
          id:             d.id,
          discipline:     disc.name,
          disciplineCode: disc.code,
          student:        student.name,
          group:          student.group,
          teacher:        teacher.name,
        })
      }
    }
    rows.sort((a, b) => a.discipline.localeCompare(b.discipline, 'ru') || a.student.localeCompare(b.student, 'ru'))
    debtTableRows.value = rows
  } finally {
    debtTableLoading.value = false
  }
}

// Подпись активных фильтров для шапки документа.
function activeFilterCaption() {
  const parts = []
  if (filterDisc.value)    parts.push(`Дисциплина: ${filterDisc.value}`)
  if (filterGroup.value)   parts.push(`Группа: ${filterGroup.value}`)
  if (filterTeacher.value) parts.push(`Преподаватель: ${filterTeacher.value}`)
  return parts.length ? parts.join('; ') : 'Все дисциплины, группы и преподаватели'
}

function todayLong() {
  return new Date().toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })
}

async function doExportExcel() {
  const XLSX = await import('xlsx')
  const wb = XLSX.utils.book_new()
  const aoa = [
    ['ЮГОРСКИЙ ГОСУДАРСТВЕННЫЙ УНИВЕРСИТЕТ'],
    ['СВЕДЕНИЯ ОБ АКАДЕМИЧЕСКИХ ЗАДОЛЖЕННОСТЯХ'],
    [],
    ['Выборка:', activeFilterCaption()],
    ['Дата формирования:', todayLong()],
    ['Всего записей:', filteredDebtRows.value.length],
    [],
    ['№', 'Дисциплина', 'Код', 'Студент', 'Группа', 'Преподаватель'],
    ...filteredDebtRows.value.map((r, i) => [i + 1, r.discipline, r.disciplineCode, r.student, r.group, r.teacher]),
  ]
  const ws = XLSX.utils.aoa_to_sheet(aoa)
  ws['!cols'] = [{ wch: 5 }, { wch: 34 }, { wch: 12 }, { wch: 32 }, { wch: 12 }, { wch: 30 }]
  ws['!merges'] = [
    { s: { r: 0, c: 0 }, e: { r: 0, c: 5 } },
    { s: { r: 1, c: 0 }, e: { r: 1, c: 5 } },
  ]
  XLSX.utils.book_append_sheet(wb, ws, 'Задолженности')
  XLSX.writeFile(wb, `Академические_задолженности_${new Date().toISOString().slice(0, 10)}.xlsx`)
}

async function doExportWord() {
  const libs = await import('docx')
  const {
    Document, Packer, Paragraph, Table, TableRow, TableCell,
    TextRun, AlignmentType, WidthType,
  } = libs

  const PT = n => n * 2
  const CM = n => Math.round(n * 567)
  const brd = { style: 'single', size: 4, color: '000000' }
  const allBorders = { top: brd, bottom: brd, left: brd, right: brd }

  const p = (text, o = {}) => new Paragraph({
    alignment: o.align ?? AlignmentType.LEFT,
    spacing: { before: CM(o.before ?? 0), after: CM(o.after ?? 0.18) },
    children: [new TextRun({ text: String(text), bold: !!o.bold, size: PT(o.size ?? 12), font: 'Times New Roman' })],
  })
  const cell = (text, o = {}) => new TableCell({
    width: o.w ? { size: o.w, type: WidthType.PERCENTAGE } : undefined,
    shading: o.shade ? { fill: 'EEEEEE' } : undefined,
    borders: allBorders,
    children: [new Paragraph({
      alignment: o.align ?? AlignmentType.LEFT,
      children: [new TextRun({ text: String(text), bold: !!o.bold, size: PT(o.size ?? 11), font: 'Times New Roman' })],
    })],
  })

  const rows = filteredDebtRows.value
  const header = new TableRow({
    tableHeader: true,
    children: [
      cell('№',            { bold: true, align: AlignmentType.CENTER, w: 6,  shade: true }),
      cell('Дисциплина',   { bold: true, w: 30, shade: true }),
      cell('Студент',      { bold: true, w: 28, shade: true }),
      cell('Группа',       { bold: true, align: AlignmentType.CENTER, w: 12, shade: true }),
      cell('Преподаватель',{ bold: true, w: 24, shade: true }),
    ],
  })
  const bodyRows = rows.map((r, i) => new TableRow({
    children: [
      cell(i + 1, { align: AlignmentType.CENTER }),
      cell(r.disciplineCode ? `${r.discipline} (${r.disciplineCode})` : r.discipline),
      cell(r.student),
      cell(r.group || '—', { align: AlignmentType.CENTER }),
      cell(r.teacher),
    ],
  }))

  const doc = new Document({
    styles: { default: { document: { run: { font: 'Times New Roman', size: PT(12) } } } },
    sections: [{
      properties: { page: { margin: { top: CM(2), right: CM(1.5), bottom: CM(2), left: CM(3) } } },
      children: [
        p('ФЕДЕРАЛЬНОЕ ГОСУДАРСТВЕННОЕ БЮДЖЕТНОЕ ОБРАЗОВАТЕЛЬНОЕ УЧРЕЖДЕНИЕ ВЫСШЕГО ОБРАЗОВАНИЯ', { align: AlignmentType.CENTER, size: 13 }),
        p('ЮГОРСКИЙ ГОСУДАРСТВЕННЫЙ УНИВЕРСИТЕТ', { align: AlignmentType.CENTER, size: 13, after: 0.3 }),
        p('СВЕДЕНИЯ ОБ АКАДЕМИЧЕСКИХ ЗАДОЛЖЕННОСТЯХ', { align: AlignmentType.CENTER, bold: true, size: 16, before: 0.3, after: 0.4 }),
        p(`Выборка: ${activeFilterCaption()}`, { size: 11 }),
        p(`Дата формирования: ${todayLong()}`, { size: 11, after: 0.35 }),
        new Table({ width: { size: 100, type: WidthType.PERCENTAGE }, rows: [header, ...bodyRows] }),
        p(`Всего задолженностей: ${rows.length}`, { bold: true, before: 0.4 }),
        p('Декан: ___________________________ /________________/', { before: 0.7 }),
        p(`Дата составления: ${new Date().toLocaleDateString('ru-RU')}`, { before: 0.25 }),
      ],
    }],
  })

  const blob = await Packer.toBlob(doc)
  const url = URL.createObjectURL(blob)
  const a = Object.assign(document.createElement('a'), {
    href: url, download: `Академические_задолженности_${new Date().toISOString().slice(0, 10)}.docx`,
  })
  document.body.appendChild(a); a.click(); document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

// ── Запланированные пересдачи (вторая карточка) ───────────────
const scheduledRetakes = ref([])   // [{id, subject, dateLabel, timeLabel, building, room}]

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
function onTeacherFocus() {
  clearTimeout(teacherBlurTimer)
  studentOpen.value = false  // закрываем студентов
  teacherOpen.value = true
}
function onTeacherBlur() {
  teacherBlurTimer = setTimeout(() => { teacherOpen.value = false }, 200)
}
function onStudentFocus() {
  clearTimeout(studentBlurTimer)
  teacherOpen.value = false  // закрываем преподавателей
  studentOpen.value = true
}
function onStudentBlur() {
  studentBlurTimer = setTimeout(() => { studentOpen.value = false }, 200)
}

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

// ── Group dropdown ────────────────────────────────────────────
const groupDropdownOpen = ref(false)
function selectGroup(g) { groupSelect.value = g; groupDropdownOpen.value = false }
function handleGroupOutside(e) { if (!e.target.closest('.group-custom-select')) groupDropdownOpen.value = false }

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

const BUILDING_RE = /^[А-Яа-яA-Za-z0-9\s\-\/\.]{1,20}$/
const ROOM_RE     = /^[А-Яа-яA-Za-z0-9\s\-\/\.]{1,20}$/

async function submitRetake() {
  submitError.value = ''
  if (!disciplineId.value)   { submitError.value = 'Выберите дисциплину'; return }
  if (!retakeDate.value)      { submitError.value = 'Укажите дату'; return }

  // Время вводится в зоне вуза (Ханты, UTC+5) → toUtcISO собирает
  // корректный UTC ISO, а не интерпретирует как зону устройства.
  const [day, month, year] = retakeDate.value.split('.')
  const scheduledAtIso = toUtcISO(`${year}-${month}-${day}`, timeString.value)
  if (new Date(scheduledAtIso) <= new Date()) { submitError.value = 'Дата и время пересдачи должны быть в будущем'; return }

  const bld = building.value.trim()
  const rm  = room.value.trim()
  if (!bld) { submitError.value = 'Укажите корпус'; return }
  if (!BUILDING_RE.test(bld)) { submitError.value = 'Некорректный номер корпуса (только буквы, цифры, до 20 символов)'; return }
  if (!rm) { submitError.value = 'Укажите аудиторию'; return }
  if (!ROOM_RE.test(rm)) { submitError.value = 'Некорректный номер аудитории (только буквы, цифры, до 20 символов)'; return }

  if (!teacherCountOk.value)  {
    submitError.value = isCommission.value
      ? 'Для комиссии нужно минимум 3 преподавателя'
      : 'Добавьте хотя бы одного преподавателя'
    return
  }

  submitting.value = true
  try {
    const { data: retake } = await retakesApi.create({
      discipline_id: disciplineId.value,
      kind: isCommission.value ? 'commission' : 'regular',
      building: bld,
      room: rm,
      scheduled_at: scheduledAtIso,
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
  if (!e.target.closest('.group-custom-select')) groupDropdownOpen.value = false
}

async function loadDashboard() {
  const [summaryRes, retakesRes, scheduledRes, disciplinesRes] = await Promise.allSettled([
    debtsApi.getSummary(),
    retakesApi.getAll({ limit: 200 }),
    retakesApi.getAll({ status: 'scheduled', limit: 100 }),
    disciplinesApi.getAll({ limit: 200 }),
  ])

  if (disciplinesRes.status === 'fulfilled') {
    disciplines.value = disciplinesRes.value.data.items ?? []
    for (const d of disciplines.value) discMap.value[d.id] = d.name || d.code
  }

  let totalDebts = 0
  if (summaryRes.status === 'fulfilled')
    totalDebts = (summaryRes.value.data ?? []).reduce((s, r) => s + (r.open_count ?? 0), 0)

  let scheduledCount = 0, inProgress = 0, completedMonth = 0
  if (retakesRes.status === 'fulfilled') {
    const items = retakesRes.value.data.items ?? []
    scheduledCount = items.filter(r => r.status === 'scheduled').length
    inProgress = items.filter(r => r.status === 'in_progress').length
    const now = new Date()
    completedMonth = items.filter(r => {
      if (r.status !== 'completed') return false
      const d = new Date(r.completed_at ?? r.updated_at ?? r.created_at)
      return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth()
    }).length
  }

  stats.value = [
    { key: 'debts',     label: 'Академические долги', value: totalDebts,     accent: '#e63c5a' },
    { key: 'scheduled', label: 'Запланировано',       value: scheduledCount, accent: '#3b3fe0' },
    { key: 'progress',  label: 'Проводится сейчас',   value: inProgress,     accent: '#f59e0b' },
    { key: 'done',      label: 'Завершено в месяце',  value: completedMonth, accent: '#10b981' },
  ]

  if (scheduledRes.status === 'fulfilled') {
    const scheduledItems = (scheduledRes.value.data.items ?? [])
    upcomingRetakes.value = scheduledItems.map(r => {
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
    // Список для панели «Запланировано» (с подписями даты/времени)
    scheduledRetakes.value = scheduledItems.map(r => {
      const p = partsInTZ(r.scheduled_at)
      return {
        id: r.id,
        subject: discMap.value[r.discipline_id] || 'Дисциплина',
        dateLabel: `${p.day}.${p.month}.${p.year}`,
        timeLabel: `${p.hour}:${p.minute}`,
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
            <StatCard
              v-for="s in stats" :key="s.key"
              :label="s.label"
              :value="s.value"
              :clickable="s.key === 'debts' || s.key === 'scheduled'"
              :active="activePanel === s.key"
              @click="togglePanel(s.key)"
            />
          </div>

          <!-- Панель: Актуальные академические долги -->
          <Transition name="panel">
            <div v-if="activePanel === 'debts'" class="debt-table-card">
              <div class="debt-table-head">
                <h3 class="debt-table-title">Актуальные академические долги</h3>
                <div class="debt-table-actions">
                  <button class="btn-exp excel" @click="doExportExcel" :disabled="debtTableLoading || !filteredDebtRows.length">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/>
                    </svg>
                    Excel
                  </button>
                  <button class="btn-exp word" @click="doExportWord" :disabled="debtTableLoading || !filteredDebtRows.length">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M9 13l2 4 2-4 2 4"/>
                    </svg>
                    Word
                  </button>
                  <button class="debt-table-close" @click="activePanel = null">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                  </button>
                </div>
              </div>

              <!-- Фильтры -->
              <div class="filters-bar">
                <FilterSelect v-model="filterDisc" :options="debtDiscs" placeholder="Все предметы" />
                <FilterSelect v-model="filterGroup" :options="debtGroups" placeholder="Все группы" />
                <FilterSelect v-model="filterTeacher" :options="debtTeachers" placeholder="Все преподаватели" />
                <button class="btn-reset" :disabled="!hasActiveFilters" @click="resetFilters">Сбросить фильтры</button>
              </div>

              <div v-if="debtTableLoading" class="debt-table-state">
                <div class="spinner" /><span>Загрузка данных…</span>
              </div>
              <EmptyPlate
                v-else-if="filteredDebtRows.length === 0"
                icon="check"
                :text="hasActiveFilters ? 'По выбранным фильтрам долгов нет' : 'Актуальных долгов нет'"
              />
              <div v-else class="debt-table-wrap">
                <table class="debt-table">
                  <thead>
                    <tr>
                      <th>Дисциплина</th>
                      <th>Студент</th>
                      <th>Группа</th>
                      <th>Преподаватель</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="r in filteredDebtRows" :key="r.id">
                      <td>
                        <div class="td-disc-name">{{ r.discipline }}</div>
                        <div v-if="r.disciplineCode" class="td-disc-code">{{ r.disciplineCode }}</div>
                      </td>
                      <td>{{ r.student }}</td>
                      <td>{{ r.group || '—' }}</td>
                      <td>{{ r.teacher }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-if="!debtTableLoading && filteredDebtRows.length > 0" class="debt-table-foot">
                Показано {{ filteredDebtRows.length }} из {{ debtTableRows.length }} записей
              </div>
            </div>
          </Transition>

          <!-- Панель: Запланированные пересдачи -->
          <Transition name="panel">
            <div v-if="activePanel === 'scheduled'" class="debt-table-card">
              <div class="debt-table-head">
                <h3 class="debt-table-title">Запланированные пересдачи</h3>
                <button class="debt-table-close" @click="activePanel = null">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                </button>
              </div>
              <EmptyPlate
                v-if="scheduledRetakes.length === 0"
                icon="calendar"
                text="Запланированных пересдач нет"
              />
              <ul v-else class="sched-list">
                <li v-for="r in scheduledRetakes" :key="r.id" class="sched-row">
                  <div class="sched-icon">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/>
                    </svg>
                  </div>
                  <div class="sched-body">
                    <span class="sched-subject">{{ r.subject }}</span>
                    <span class="sched-meta">
                      {{ r.dateLabel }} · {{ r.timeLabel }}
                      <template v-if="r.building || r.room">· {{ r.building ? `корп. ${r.building}` : '' }}{{ r.room ? ` ауд. ${r.room}` : '' }}</template>
                    </span>
                  </div>
                </li>
              </ul>
            </div>
          </Transition>

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

.group-custom-select { position: relative; flex: 1; }
.group-select-trigger {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; height: 40px; padding: 0 12px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; text-align: left;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.group-select-trigger:hover:not(:disabled) { border-color: #a0a3b1; }
.group-select-trigger:disabled { opacity: .5; cursor: not-allowed; background: #f9fafb; }
.group-custom-select.open .group-select-trigger { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.group-select-trigger .placeholder { color: var(--ink-soft); }
.select-arrow { width: 14px; height: 14px; color: var(--ink-soft); flex-shrink: 0; transition: transform .2s var(--ease); }
.group-custom-select.open .select-arrow { transform: rotate(180deg); }

.group-select-dropdown {
  position: absolute; top: calc(100% + 2px); left: 0; right: 0; z-index: 100;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.14);
  max-height: 200px; overflow-y: auto;
}
.group-select-dropdown::-webkit-scrollbar { width: 4px; }
.group-select-dropdown::-webkit-scrollbar-track { background: transparent; }
.group-select-dropdown::-webkit-scrollbar-thumb { background: #c5c8d4; border-radius: 4px; }
.group-select-option {
  display: block; width: 100%; text-align: left; padding: 9px 14px;
  background: none; border: none; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s;
}
.group-select-option:hover { background: rgba(59,63,224,.07); }
.group-select-option.selected { color: var(--brand); font-weight: 600; }
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

/* ── Panel transition ── */
.panel-enter-active { transition: opacity .2s var(--ease), transform .2s var(--ease); }
.panel-leave-active { transition: opacity .15s var(--ease), transform .15s var(--ease); }
.panel-enter-from, .panel-leave-to { opacity: 0; transform: translateY(-8px); }

/* ── Debt table card ── */
.debt-table-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); overflow: hidden;
  border: 1.5px solid rgba(59,63,224,.15); margin-bottom: 24px;
}
.debt-table-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 20px; border-bottom: 1px solid var(--line);
  gap: 10px; flex-wrap: wrap;
}
.debt-table-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 14px; font-weight: 700; color: #3C38B6; margin: 0; white-space: nowrap;
}
.debt-table-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

/* ── Export buttons (как в ведомости) ── */
.btn-exp {
  display: inline-flex; align-items: center; gap: 6px;
  height: 32px; padding: 0 12px; border-radius: 8px;
  font: 600 12px/1 'Inter', sans-serif; cursor: pointer; white-space: nowrap;
  background: #fff; transition: background .15s, border-color .15s;
}
.btn-exp svg { width: 14px; height: 14px; flex-shrink: 0; }
.btn-exp:disabled { opacity: .4; cursor: not-allowed; }
.btn-exp.excel { border: 1.5px solid #1a7340; color: #1a7340; }
.btn-exp.excel:hover:not(:disabled) { background: rgba(26,115,64,.07); }
.btn-exp.word  { border: 1.5px solid #1a56a0; color: #1a56a0; }
.btn-exp.word:hover:not(:disabled)  { background: rgba(26,86,160,.07); }

/* ── Фильтры ── */
.filters-bar {
  display: flex; align-items: center; gap: 10px; flex-wrap: wrap;
  padding: 12px 20px; border-bottom: 1px solid var(--line); background: #fafbfc;
}
.btn-reset {
  height: 36px; padding: 0 14px; flex-shrink: 0;
  border: 1.5px solid var(--line); border-radius: 8px;
  background: #fff; font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; white-space: nowrap;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-reset:hover:not(:disabled) { border-color: var(--brand); color: var(--brand); background: rgba(59,63,224,.04); }
.btn-reset:disabled { opacity: .4; cursor: not-allowed; }

.debt-table-close {
  width: 28px; height: 28px; border-radius: 7px; flex-shrink: 0;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s;
}
.debt-table-close:hover { background: var(--bg); color: var(--ink); }
.debt-table-close svg { width: 14px; height: 14px; }

.debt-table-state {
  display: flex; align-items: center; justify-content: center; gap: 10px;
  padding: 32px; font: 13px/1 'Inter', sans-serif; color: var(--ink-soft);
}

.debt-table-wrap { overflow-x: auto; max-height: 480px; overflow-y: auto; }
.debt-table-wrap::-webkit-scrollbar { width: 4px; height: 4px; }
.debt-table-wrap::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }

.debt-table { width: 100%; border-collapse: collapse; font: 13px/1.4 'Inter', sans-serif; }
.debt-table thead tr { position: sticky; top: 0; z-index: 2; }
.debt-table th {
  background: #f8f9fb; padding: 9px 14px;
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-align: left; white-space: nowrap; border-bottom: 1px solid var(--line);
}
.debt-table td { padding: 10px 14px; border-bottom: 1px solid var(--line); color: var(--ink); vertical-align: middle; text-align: left; }
.debt-table .td-disc-name, .debt-table .td-disc-code { text-align: left; }
.debt-table tbody tr:last-child td { border-bottom: none; }
.debt-table tbody tr:hover td { background: rgba(59,63,224,.025); }

.td-disc-name { font-weight: 500; }
.td-disc-code { font-size: 11px; color: var(--ink-soft); margin-top: 2px; }

.debt-table-foot {
  padding: 8px 20px; border-top: 1px solid var(--line);
  font: 11px/1 'Inter', sans-serif; color: var(--ink-soft); text-align: left;
}

/* ── Запланированные пересдачи ── */
.sched-list { list-style: none; margin: 0; padding: 0; max-height: 480px; overflow-y: auto; }
.sched-list::-webkit-scrollbar { width: 4px; }
.sched-list::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }
.sched-row {
  display: flex; align-items: center; gap: 12px;
  padding: 12px 20px; border-bottom: 1px solid var(--line);
  transition: background .12s;
}
.sched-row:last-child { border-bottom: none; }
.sched-row:hover { background: rgba(59,63,224,.025); }
.sched-icon {
  width: 36px; height: 36px; border-radius: 9px; flex-shrink: 0;
  background: rgba(59,63,224,.08); color: var(--brand);
  display: grid; place-items: center;
}
.sched-icon svg { width: 18px; height: 18px; }
.sched-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; text-align: left; }
.sched-subject { font: 500 13px/1.4 'Inter', sans-serif; color: var(--ink); }
.sched-meta { font: 12px/1.4 'Inter', sans-serif; color: var(--ink-soft); }

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
