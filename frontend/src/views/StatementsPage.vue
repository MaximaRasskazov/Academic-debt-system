<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { directoryApi } from '../api/directory'
import { usersApi } from '../api/users'

const auth = useAuthStore()
const isTeacher = computed(() => auth.isTeacher)
const sidebarOpen = ref(false)

/* ─── Логика ведомости ──────────────────────────────────────────
 *
 * В ведомость попадают только пересдачи, которые НАЧАЛИСЬ или уже
 * ЗАВЕРШИЛИСЬ (статусы in_progress / completed). Пока пересдача
 * "Назначена" (scheduled) — оценивать нечего, она в ведомость не
 * выводится. Это совпадает с бэкендом: GradeStudent разрешает оценку
 * только в scheduled|in_progress, а scheduled мы и так не показываем.
 *
 * Оценку (2..5) ставит преподаватель-участник. Каждая оценка
 * применяется немедленно через PATCH /api/retakes/:id/students/:uid/grade:
 * это атомарно закрывает связанный долг и фиксирует дату (graded_at).
 * Бэкенд запрещает переставлять уже выставленную оценку, поэтому в UI
 * проставленная оценка блокируется.
 *
 * Если по завершении пересдачи оценка так и не выставлена — долг
 * остаётся открытым; в ведомости такая строка показывается как
 * "Долг остался".
 *
 * Декан видит ведомость только на чтение + выгрузка Word/Excel.
 */

/* ─── State ─────────────────────────────────────────────────── */
const allRetakes = ref([])        // нормализованные пересдачи
const loading    = ref(true)
const loadError  = ref('')
const discMap    = ref({})         // discipline_id → { name, code }
const userMap    = ref({})         // user_id → { lastName, firstName, middleName, group }

/* ─── Left panel ─────────────────────────────────────────────── */
const searchQuery  = ref('')
const filterStatus = ref('all')

const filteredRetakes = computed(() =>
  allRetakes.value.filter(r => {
    const q = searchQuery.value.toLowerCase()
    return (!q || r.subject.toLowerCase().includes(q)) &&
           (filterStatus.value === 'all' || r.status === filterStatus.value)
  })
)

/* ─── Selection ──────────────────────────────────────────────── */
const selectedId     = ref(null)
const selectedRetake = computed(() => allRetakes.value.find(r => r.id === selectedId.value) ?? null)
const detailLoading  = ref(false)

// Оценки редактируемы только преподавателем и пока ведомость не
// зафиксирована. completed-пересдачу бэкенд грейдить запрещает,
// поэтому на ней оценки read-only. То есть редактируемо = teacher
// + статус in_progress + оценка ещё не стоит.
function canGrade(retake) {
  return isTeacher.value && retake && retake.status === 'in_progress'
}

watch(selectedId, async (id) => {
  if (!id) return
  await loadParticipants(id)
})

/* ─── Grade ──────────────────────────────────────────────────── */
const gradingId   = ref(null)   // student_id, по которому идёт запрос
const gradeError  = ref('')
const saveSuccess = ref(false)

async function setGrade(retake, student, grade) {
  if (!canGrade(retake)) return
  if (student.result !== null) return        // переставить нельзя
  if (gradingId.value) return                 // не плодим параллельные

  gradingId.value = student.id
  gradeError.value = ''
  try {
    await retakesApi.gradeStudent(retake.id, student.id, grade)
    // Оптимистично фиксируем результат и дату — повторно грузить не нужно
    student.result   = grade
    student.gradedAt = new Date().toISOString()
    saveSuccess.value = true
    setTimeout(() => (saveSuccess.value = false), 2000)
  } catch (e) {
    gradeError.value = e.response?.data?.message || e.response?.data?.error || 'Не удалось выставить оценку'
    setTimeout(() => (gradeError.value = ''), 4000)
  } finally {
    gradingId.value = null
  }
}

/* ─── Export panel ───────────────────────────────────────────── */
const exportPanelOpen = ref(false)
const expSubjects     = ref([])
const expDateFrom     = ref('')
const expDateTo       = ref('')
const expStatus       = ref('all')

const allSubjects = computed(() => [...new Set(allRetakes.value.map(r => r.subject))])

function toggleSubject(s) {
  const i = expSubjects.value.indexOf(s)
  i === -1 ? expSubjects.value.push(s) : expSubjects.value.splice(i, 1)
}

const exportFiltered = computed(() =>
  allRetakes.value.filter(r => {
    const okSub  = !expSubjects.value.length || expSubjects.value.includes(r.subject)
    const okSt   = expStatus.value === 'all' || r.status === expStatus.value
    const okFrom = !expDateFrom.value || r.date >= expDateFrom.value
    const okTo   = !expDateTo.value   || r.date <= expDateTo.value
    return okSub && okSt && okFrom && okTo
  })
)

/* ─── Helpers ────────────────────────────────────────────────── */
const STATUS_MAP = {
  in_progress: { label: 'Проводится', bg: 'rgba(245,158,11,.12)', color: '#d97706' },
  completed:   { label: 'Завершена',  bg: 'rgba(5,150,105,.1)',   color: '#059669' },
}

function fullName(s)  { return [s.lastName, s.firstName, s.middleName].filter(Boolean).join(' ') }
function typeLabel(t) { return t === 'commission' ? 'С комиссией' : 'Обычная' }

// "Оценено" = либо стоит оценка, либо (для завершённой) долг остался —
// то есть результат по студенту окончательно определён.
function gradeCount(arr) { return arr.filter(s => s.result !== null).length }
function todayStr()      { return new Date().toISOString().split('T')[0] }

function resultLabel(result, status) {
  if (result === 0) return 'Н/я'
  if (result !== null && result !== undefined) return String(result)
  return status === 'completed' ? 'Долг' : '—'
}

function resultColor(result, status) {
  const map = { 0: '#6b7280', 2: '#dc2626', 3: '#d97706', 4: '#3b82f6', 5: '#059669' }
  if (result !== null && result !== undefined) return map[result] || '#b0b3be'
  return status === 'completed' ? '#dc2626' : '#b0b3be'
}

function fmtDateTime(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
       + ' ' + d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

function formatLong(dateStr) {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('ru-RU', {
    day: 'numeric', month: 'long', year: 'numeric',
  })
}

function formatShort(dateStr) {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric',
  })
}

/* ─── Загрузка данных ────────────────────────────────────────── */

// Пересдачи, попадающие в ведомость: только начавшиеся/завершённые.
const VEDOMOST_STATUSES = ['in_progress', 'completed']

function normalizeRetake(r) {
  const d = new Date(r.scheduled_at)
  const disc = discMap.value[r.discipline_id]
  return {
    id:        r.id,
    disciplineId: r.discipline_id,
    subject:   disc?.name || disc?.code || 'Дисциплина',
    code:      disc?.code || '',
    date:      d.toISOString().split('T')[0],
    time:      `${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`,
    duration:  r.duration_minutes,
    building:  r.building,
    room:      r.room,
    type:      r.kind === 'commission' ? 'commission' : 'normal',
    status:    r.status,
    completedAt: r.completed_at || null,
    teachers:  [],          // подтянем из participants при открытии
    students:  [],          // подтянем из participants при открытии
    _loaded:   false,
  }
}

async function loadData() {
  loading.value = true
  loadError.value = ''
  try {
    // 1. Справочник дисциплин (для названий)
    const discsRes = await disciplinesApi.getAll({ limit: 500 }).catch(() => null)
    if (discsRes) {
      for (const d of (discsRes.data.items ?? discsRes.data ?? [])) {
        discMap.value[d.id] = { name: d.name, code: d.code }
      }
    }

    // 2. Пересдачи. Декан — все, преподаватель — только свои.
    let rows = []
    if (auth.isDean || auth.isAdmin) {
      // Тянем in_progress и completed двумя запросами (бэк фильтрует по одному статусу)
      const [ipRes, cmpRes] = await Promise.allSettled([
        retakesApi.getAll({ status: 'in_progress', limit: 200 }),
        retakesApi.getAll({ status: 'completed', limit: 200 }),
      ])
      if (ipRes.status === 'fulfilled')  rows.push(...(ipRes.value.data.items ?? []))
      if (cmpRes.status === 'fulfilled') rows.push(...(cmpRes.value.data.items ?? []))
    } else {
      const myRes = await retakesApi.getMy().catch(() => null)
      const items = myRes ? (myRes.data.items ?? myRes.data ?? []) : []
      rows = items.filter(r => VEDOMOST_STATUSES.includes(r.status))
    }

    allRetakes.value = rows
      .map(normalizeRetake)
      .sort((a, b) => (a.date < b.date ? 1 : -1))
  } catch (e) {
    loadError.value = e.response?.data?.message || 'Не удалось загрузить ведомости'
  } finally {
    loading.value = false
  }
}

// Подгрузка участников выбранной пересдачи + резолв ФИО.
async function loadParticipants(retakeId) {
  const retake = allRetakes.value.find(r => r.id === retakeId)
  if (!retake || retake._loaded) return

  detailLoading.value = true
  try {
    const partRes = await retakesApi.getParticipants(retakeId).catch(() => null)
    const parts = partRes ? (partRes.data.items ?? partRes.data ?? []) : []

    // ФИО студентов: преподаватель не имеет users.view, поэтому берём
    // из directoryApi.listDebtors по дисциплине (он отдаёт ФИО+группу).
    // Декан может тянуть users напрямую, но listDebtors проще и единообразно.
    await ensureNamesForDiscipline(retake.disciplineId)

    const teachers = []
    const students = []
    for (const p of parts) {
      const u = userMap.value[p.user_id]
      const name = u ? [u.lastName, u.firstName, u.middleName].filter(Boolean).join(' ') : p.user_id
      if (p.kind === 'student') {
        students.push({
          id:        p.user_id,
          lastName:  u?.lastName  || '',
          firstName: u?.firstName || '',
          middleName:u?.middleName|| '',
          group:     u?.group     || '',
          result:    p.grade ?? null,
          gradedAt:  p.graded_at || null,
          debtId:    p.debt_id || null,
        })
      } else {
        teachers.push(name)
      }
    }
    // Студентов сортируем по фамилии для ведомости
    students.sort((a, b) => fullName(a).localeCompare(fullName(b), 'ru'))

    retake.teachers = teachers
    retake.students = students
    retake._loaded  = true
  } finally {
    detailLoading.value = false
  }
}

// Заполняет userMap ФИО студентов-должников по дисциплине.
// Кеширует по дисциплине, чтобы не дёргать listDebtors повторно.
const _discNamesLoaded = new Set()
async function ensureNamesForDiscipline(disciplineId) {
  if (_discNamesLoaded.has(disciplineId)) return
  _discNamesLoaded.add(disciplineId)

  // Декан — полный справочник пользователей (есть users.view)
  if (auth.isDean || auth.isAdmin) {
    if (!_allUsersLoaded) {
      const res = await usersApi.getAll({ limit: 1000 }).catch(() => null)
      if (res) {
        for (const u of (res.data.items ?? [])) {
          userMap.value[u.id] = {
            lastName: u.last_name, firstName: u.first_name,
            middleName: u.middle_name, group: u.group_name || '',
          }
        }
      }
      _allUsersLoaded = true
    }
    return
  }

  // Преподаватель — студенты через listDebtors, преподаватели через listTeachers
  const [debtorsRes, teachersRes] = await Promise.allSettled([
    directoryApi.listDebtors(disciplineId),
    directoryApi.listTeachers(),
  ])
  if (debtorsRes.status === 'fulfilled') {
    for (const s of (debtorsRes.value.data.items ?? [])) {
      userMap.value[s.student_id] = {
        lastName: s.last_name, firstName: s.first_name,
        middleName: s.middle_name, group: s.group_name || '',
      }
    }
  }
  if (teachersRes.status === 'fulfilled') {
    for (const t of (teachersRes.value.data.items ?? [])) {
      userMap.value[t.id] = {
        lastName: t.last_name, firstName: t.first_name,
        middleName: t.middle_name, group: '',
      }
    }
  }
}
let _allUsersLoaded = false

onMounted(loadData)

/* ─── Excel export ───────────────────────────────────────────── */
async function doExcelOne(retake, students) {
  const XLSX = await import('xlsx')
  const wb   = XLSX.utils.book_new()
  appendSheet(XLSX, wb, retake, students)
  XLSX.writeFile(wb, `Ведомость_${retake.subject}_${retake.date}.xlsx`)
}

async function doExcelAll() {
  // Подгружаем участников для всех попадающих в выборку пересдач —
  // они грузятся лениво при открытии, а для экспорта нужны все.
  await Promise.all(exportFiltered.value.map(r => loadParticipants(r.id)))
  const XLSX = await import('xlsx')
  const wb   = XLSX.utils.book_new()
  exportFiltered.value.forEach((r, i) => {
    const name = r.subject.slice(0, 26).replace(/[*?:/\\[\]]/g, '_')
      + (r.subject.length > 26 ? `_${i + 1}` : '')
    appendSheet(XLSX, wb, r, r.students, name)
  })
  XLSX.writeFile(wb, 'Ведомость_все_пересдачи.xlsx')
}

function appendSheet(XLSX, wb, retake, students, sheetName) {
  const rows = [
    ['ВЕДОМОСТЬ ПЕРЕСДАЧИ'],
    [],
    ['Дисциплина:',      retake.subject],
    ['Дата:',            formatLong(retake.date)],
    ['Время:',           retake.time],
    ['Место:',           `Корпус ${retake.building}, аудитория ${retake.room}`],
    ['Тип пересдачи:',   typeLabel(retake.type)],
    ['Статус:',          STATUS_MAP[retake.status]?.label || retake.status],
    ['Преподаватель(и):', retake.teachers.join('; ')],
    [],
    ['№', 'ФИО студента', 'Группа', 'Результат'],
    ...students.map((s, i) => [i + 1, fullName(s), s.group, resultLabel(s.result, retake.status)]),
    [],
    ['Преподаватель:', retake.teachers[0] || ''],
    ['Дата составления:', formatShort(todayStr())],
  ]
  const ws = XLSX.utils.aoa_to_sheet(rows)
  ws['!cols']   = [{ wch: 6 }, { wch: 36 }, { wch: 12 }, { wch: 10 }]
  ws['!merges'] = [{ s: { r: 0, c: 0 }, e: { r: 0, c: 3 } }]
  XLSX.utils.book_append_sheet(wb, ws, (sheetName || retake.subject).slice(0, 31))
}

/* ─── Word export ────────────────────────────────────────────── */
async function doWordOne(retake, students) {
  const libs = await import('docx')
  const doc  = buildWordDoc(libs, [{ retake, students }])
  const blob = await libs.Packer.toBlob(doc)
  downloadBlob(blob, `Ведомость_${retake.subject}_${retake.date}.docx`)
}

async function doWordAll() {
  await Promise.all(exportFiltered.value.map(r => loadParticipants(r.id)))
  const libs  = await import('docx')
  const items = exportFiltered.value.map(r => ({ retake: r, students: r.students }))
  const doc   = buildWordDoc(libs, items)
  const blob  = await libs.Packer.toBlob(doc)
  downloadBlob(blob, 'Ведомость_все_пересдачи.docx')
}

function buildWordDoc(libs, items) {
  const {
    Document, Paragraph, Table, TableRow, TableCell,
    TextRun, AlignmentType, WidthType,
  } = libs

  const PT  = n => n * 2
  const CM  = n => Math.round(n * 567)

  const brd = { style: 'single', size: 4, color: '000000' }
  const allBorders = { top: brd, bottom: brd, left: brd, right: brd }

  function p(text, opts = {}) {
    return new Paragraph({
      alignment: opts.align ?? AlignmentType.LEFT,
      pageBreakBefore: !!opts.pageBreak,
      spacing: { before: CM(opts.before ?? 0), after: CM(opts.after ?? 0.18) },
      children: [new TextRun({
        text: String(text),
        bold:      !!opts.bold,
        size:      PT(opts.size ?? 12),
        font:      'Times New Roman',
        underline: opts.underline ? {} : undefined,
      })],
    })
  }

  function cell(text, opts = {}) {
    return new TableCell({
      columnSpan: opts.span,
      width: opts.w ? { size: opts.w, type: WidthType.PERCENTAGE } : undefined,
      shading: opts.shade ? { fill: 'EEEEEE' } : undefined,
      borders: allBorders,
      children: [new Paragraph({
        alignment: opts.align ?? AlignmentType.LEFT,
        children: [new TextRun({
          text:  String(text),
          bold:  !!opts.bold,
          size:  PT(opts.size ?? 11),
          font:  'Times New Roman',
        })],
      })],
    })
  }

  const children = []

  items.forEach(({ retake, students }, idx) => {
    if (idx > 0) children.push(p('', { pageBreak: true }))

    /* Шапка */
    children.push(
      p('ФЕДЕРАЛЬНОЕ ГОСУДАРСТВЕННОЕ БЮДЖЕТНОЕ ОБРАЗОВАТЕЛЬНОЕ УЧРЕЖДЕНИЕ ВЫСШЕГО ОБРАЗОВАНИЯ', { align: AlignmentType.CENTER, size: 14 }),
      p('ЮГОРСКИЙ ГОСУДАРСТВЕННЫЙ УНИВЕРСИТЕТ', { align: AlignmentType.CENTER, size: 14, after: 0.3 }),
      p('ВЕДОМОСТЬ ПЕРЕСДАЧИ', { align: AlignmentType.CENTER, bold: true, size: 16, before: 0.3, after: 0.5 }),
    )

    /* Таблица с информацией о пересдаче */
    children.push(new Table({
      width: { size: 100, type: WidthType.PERCENTAGE },
      rows: [
        new TableRow({ children: [
          cell('Дисциплина:', { bold: true, shade: true, w: 26 }),
          cell(retake.subject, { bold: true, span: 3 }),
        ]}),
        new TableRow({ children: [
          cell('Дата:', { bold: true, shade: true }),
          cell(formatLong(retake.date), { w: 28 }),
          cell('Время:', { bold: true, shade: true, w: 13 }),
          cell(retake.time, { w: 13 }),
        ]}),
        new TableRow({ children: [
          cell('Место проведения:', { bold: true, shade: true }),
          cell(`Корпус ${retake.building}, аудитория ${retake.room}`, { span: 3 }),
        ]}),
        new TableRow({ children: [
          cell('Тип пересдачи:', { bold: true, shade: true }),
          cell(typeLabel(retake.type)),
          cell('Статус:', { bold: true, shade: true }),
          cell(STATUS_MAP[retake.status]?.label || retake.status),
        ]}),
        new TableRow({ children: [
          cell('Преподаватель(и):', { bold: true, shade: true }),
          cell(retake.teachers.join(', '), { span: 3 }),
        ]}),
      ],
    }))

    /* Таблица студентов */
    children.push(p('Список студентов:', { bold: true, before: 0.45, after: 0.2 }))
    children.push(new Table({
      width: { size: 100, type: WidthType.PERCENTAGE },
      rows: [
        new TableRow({
          tableHeader: true,
          children: [
            cell('№',              { bold: true, align: AlignmentType.CENTER, w: 5,  shade: true }),
            cell('ФИО студента',   { bold: true,                              w: 43, shade: true }),
            cell('Группа',         { bold: true, align: AlignmentType.CENTER, w: 14, shade: true }),
            cell('Результат',      { bold: true, align: AlignmentType.CENTER, w: 13, shade: true }),
            cell('Подпись',        { bold: true, align: AlignmentType.CENTER, w: 25, shade: true }),
          ],
        }),
        ...students.map((s, i) => new TableRow({
          children: [
            cell(i + 1,              { align: AlignmentType.CENTER }),
            cell(fullName(s)),
            cell(s.group,            { align: AlignmentType.CENTER }),
            cell(resultLabel(s.result, retake.status), { align: AlignmentType.CENTER, bold: true }),
            cell(''),
          ],
        })),
      ],
    }))

    /* Подпись */
    children.push(
      p(`Преподаватель: ___________________________ / ${retake.teachers[0] || ''} /`, { before: 0.55 }),
      p(`Дата составления: ${formatShort(todayStr())}`, { before: 0.2 }),
    )
  })

  return new Document({
    styles: {
      default: { document: { run: { font: 'Times New Roman', size: PT(12) } } },
    },
    sections: [{
      properties: {
        page: { margin: { top: CM(2), right: CM(1.5), bottom: CM(2), left: CM(3) } },
      },
      children,
    }],
  })
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a   = Object.assign(document.createElement('a'), { href: url, download: filename })
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
</script>

<template>
  <div class="stmt-root">
    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-stmt">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <!-- Заголовок страницы -->
      <div class="page-bar">
        <div class="page-bar-left">
          <h1 class="page-title">Ведомость</h1>
        </div>
        <button class="btn-all-export" @click="exportPanelOpen = true">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
          </svg>
          Экспорт всех
        </button>
      </div>

      <main class="main">

        <!-- ── Левая панель ── -->
        <aside class="left-col">
          <div class="left-card">
            <div class="left-head">
              <span class="panel-title">Пересдачи</span>
              <span class="count-chip">{{ filteredRetakes.length }}</span>
            </div>

            <div class="search-wrap">
              <svg class="search-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
              </svg>
              <input
                class="search-input"
                type="text"
                v-model="searchQuery"
                placeholder="Поиск по предмету..."
              />
            </div>

            <div class="chips">
              <button class="chip" :class="{ active: filterStatus === 'all' }"         @click="filterStatus = 'all'">Все</button>
              <button class="chip" :class="{ active: filterStatus === 'in_progress' }" @click="filterStatus = 'in_progress'">Проводится</button>
              <button class="chip" :class="{ active: filterStatus === 'completed' }"   @click="filterStatus = 'completed'">Завершена</button>
            </div>

            <div class="retakes-list">
              <div v-if="loading" class="empty-list">
                <div class="spinner-inline" />
                Загрузка…
              </div>

              <div
                v-for="r in filteredRetakes"
                :key="r.id"
                class="r-item"
                :class="{ selected: selectedId === r.id }"
                @click="selectedId = r.id"
              >
                <div class="r-subject">{{ r.subject }}</div>
                <div class="r-meta">{{ formatShort(r.date) }} · {{ r.time }}</div>
                <div class="r-meta">Корп. {{ r.building }}, ауд. {{ r.room }}</div>
                <div class="r-footer">
                  <span
                    class="status-badge"
                    :style="{ background: STATUS_MAP[r.status]?.bg, color: STATUS_MAP[r.status]?.color }"
                  >{{ STATUS_MAP[r.status]?.label }}</span>
                </div>
              </div>

              <div v-if="!loading && !filteredRetakes.length" class="empty-list">
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="#d7d9e0" stroke-width="1.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
                Нет пересдач в ведомости
              </div>
            </div>
          </div>
        </aside>

        <!-- ── Правая панель ── -->
        <section class="right-col">

          <!-- Пустое состояние -->
          <div v-if="!selectedRetake" class="empty-state">
            <svg width="72" height="72" viewBox="0 0 24 24" fill="none" stroke="#d7d9e0" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2"/>
              <rect x="9" y="3" width="6" height="4" rx="2"/>
              <path d="M9 12h6M9 16h4"/>
            </svg>
            <p class="empty-title">Выберите пересдачу</p>
            <p class="empty-sub">Нажмите на карточку слева,<br>чтобы открыть список студентов</p>
          </div>

          <!-- Детальный просмотр -->
          <template v-else>

            <!-- Карточка информации о пересдаче -->
            <div class="info-card">
              <div class="info-head">
                <h2 class="retake-title">{{ selectedRetake.subject }}</h2>
                <span
                  class="status-badge lg"
                  :style="{ background: STATUS_MAP[selectedRetake.status]?.bg, color: STATUS_MAP[selectedRetake.status]?.color }"
                >{{ STATUS_MAP[selectedRetake.status]?.label }}</span>
              </div>

              <div class="info-grid">
                <div class="info-row">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/>
                  </svg>
                  <span>{{ formatLong(selectedRetake.date) }}, {{ selectedRetake.time }}</span>
                </div>
                <div class="info-row">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
                  </svg>
                  <span>{{ selectedRetake.duration }} мин.</span>
                </div>
                <div class="info-row">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0118 0z"/><circle cx="12" cy="10" r="3"/>
                  </svg>
                  <span>Корп. {{ selectedRetake.building }}, ауд. {{ selectedRetake.room }}</span>
                </div>
                <div class="info-row">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2"/><rect x="9" y="3" width="6" height="4" rx="2"/>
                  </svg>
                  <span>{{ typeLabel(selectedRetake.type) }}</span>
                </div>
                <div class="info-row info-row-full">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/>
                  </svg>
                  <span>{{ selectedRetake.teachers.join(', ') }}</span>
                </div>
              </div>

              <div class="info-footer">
                <div class="export-group">
                  <button class="btn-exp excel" @click="doExcelOne(selectedRetake, selectedRetake.students)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/>
                    </svg>
                    Excel
                  </button>
                  <button class="btn-exp word" @click="doWordOne(selectedRetake, selectedRetake.students)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M9 13l2 4 2-4 2 4"/>
                    </svg>
                    Word
                  </button>
                </div>
              </div>
            </div>

            <!-- Карточка со студентами -->
            <div class="students-card">
              <div class="students-head">
                <h3 class="students-title">Список студентов</h3>
                <span class="progress-text">
                  Оценено:&nbsp;<strong>{{ gradeCount(selectedRetake.students) }}</strong>&nbsp;из&nbsp;<strong>{{ selectedRetake.students.length }}</strong>
                </span>
              </div>

              <!-- Подсказка о режиме редактирования -->
              <div v-if="canGrade(selectedRetake)" class="grade-note grade-note--active">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 013 3L7 19l-4 1 1-4z"/></svg>
                Пересдача идёт — выставьте оценки студентам.
              </div>
              <div v-else-if="isTeacher && selectedRetake.status === 'completed'" class="grade-note">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/></svg>
                Пересдача завершена. Ведомость доступна только для просмотра и выгрузки.
              </div>

              <div v-if="detailLoading" class="detail-loading">
                <div class="spinner-inline" /> Загрузка списка студентов…
              </div>

              <div v-else class="table-wrap">
                <table class="tbl">
                  <colgroup>
                    <col style="width:46px">
                    <col>
                    <col style="width:104px">
                    <col style="width:280px">
                  </colgroup>
                  <thead>
                    <tr>
                      <th>№</th>
                      <th>ФИО студента</th>
                      <th>Группа</th>
                      <th>Результат</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="(s, i) in selectedRetake.students"
                      :key="s.id"
                    >
                      <td class="td-num">{{ i + 1 }}</td>
                      <td class="td-name">{{ fullName(s) }}</td>
                      <td class="td-group">{{ s.group }}</td>
                      <td class="td-grade">
                        <!-- Преподаватель, пересдача идёт, оценка ещё не стоит → можно выставить -->
                        <div v-if="canGrade(selectedRetake) && s.result === null" class="grade-picker">
                          <button
                            v-for="g in [2, 3, 4, 5]"
                            :key="g"
                            class="g-btn"
                            :class="`g${g}`"
                            :disabled="gradingId !== null"
                            @click="setGrade(selectedRetake, s, g)"
                          >{{ g }}</button>
                          <button
                            class="g-btn g-na"
                            :disabled="gradingId !== null"
                            @click="setGrade(selectedRetake, s, 0)"
                          >Н/я</button>
                          <span v-if="gradingId === s.id" class="grade-spinner" />
                        </div>
                        <!-- Оценка проставлена ИЛИ режим только для чтения -->
                        <div v-else class="result-cell">
                          <span
                            class="result-text"
                            :style="{ color: resultColor(s.result, selectedRetake.status), fontWeight: 600 }"
                          >{{ resultLabel(s.result, selectedRetake.status) }}</span>
                          <span v-if="s.gradedAt" class="result-date">{{ fmtDateTime(s.gradedAt) }}</span>
                        </div>
                      </td>
                    </tr>
                    <tr v-if="!selectedRetake.students.length">
                      <td colspan="4" class="empty-cell">На пересдачу не записаны студенты</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div v-if="gradeError" class="grade-error-bar">{{ gradeError }}</div>
              <div v-else-if="saveSuccess" class="table-footer">
                <span class="save-ok">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><polyline points="20 6 9 17 4 12"/></svg>
                  Оценка выставлена, долг закрыт
                </span>
              </div>
            </div>

          </template>
        </section>
      </main>
    </div>

    <!-- ── Панель «Экспорт всех» ── -->
    <Teleport to="body">
      <div class="ep-overlay" :class="{ active: exportPanelOpen }" @click="exportPanelOpen = false" />

      <div class="ep" :class="{ open: exportPanelOpen }">
        <div class="ep-head">
          <h3 class="ep-title">Экспорт ведомостей</h3>
          <button class="ep-close" @click="exportPanelOpen = false">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <div class="ep-body">

          <div class="ep-field">
            <label class="ep-label">Дисциплина</label>
            <div class="ep-subjects">
              <label v-for="s in allSubjects" :key="s" class="ep-check">
                <input type="checkbox" :checked="expSubjects.includes(s)" @change="toggleSubject(s)" />
                <span class="ep-check-text">{{ s }}</span>
              </label>
            </div>
            <p v-if="!expSubjects.length" class="ep-hint-gray">Не выбрано — экспортируются все</p>
          </div>

          <div class="ep-field">
            <label class="ep-label">Период</label>
            <div class="ep-daterange">
              <div class="ep-date-wrap">
                <span class="ep-date-lbl">С</span>
                <input type="date" class="ep-input" v-model="expDateFrom" />
              </div>
              <div class="ep-date-wrap">
                <span class="ep-date-lbl">По</span>
                <input type="date" class="ep-input" v-model="expDateTo" />
              </div>
            </div>
          </div>

          <div class="ep-field">
            <label class="ep-label">Статус</label>
            <div class="ep-chips">
              <button class="chip" :class="{ active: expStatus === 'all' }"         @click="expStatus = 'all'">Все</button>
              <button class="chip" :class="{ active: expStatus === 'in_progress' }" @click="expStatus = 'in_progress'">Проводится</button>
              <button class="chip" :class="{ active: expStatus === 'completed' }"   @click="expStatus = 'completed'">Завершена</button>
            </div>
          </div>

          <div class="ep-count">
            Найдено: <strong>{{ exportFiltered.length }}</strong> пересдач
            <span v-if="exportFiltered.length">
              · {{ exportFiltered.reduce((n, r) => n + r.students.length, 0) }} студентов
            </span>
          </div>

        </div>

        <div class="ep-footer">
          <button
            class="btn-exp-lg excel"
            :disabled="!exportFiltered.length"
            @click="doExcelAll"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/>
            </svg>
            Скачать Excel
          </button>
          <button
            class="btn-exp-lg word"
            :disabled="!exportFiltered.length"
            @click="doWordAll"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M9 13l2 4 2-4 2 4"/>
            </svg>
            Скачать Word
          </button>
        </div>
      </div>
    </Teleport>

  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

/* ── Root variables ─────────────────────────────────────────── */
.stmt-root {
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

.page-stmt {
  min-height: 100dvh;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

/* ── Page bar ────────────────────────────────────────────────── */
.page-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px 0;
  gap: 16px;
}
.page-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 20px; font-weight: 700;
  color: #3C38B6; margin: 0;
}

.btn-all-export {
  display: flex; align-items: center; gap: 7px;
  padding: 0 18px; height: 38px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 6px 18px -6px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
  white-space: nowrap;
}
.btn-all-export:hover { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }

/* ── Main grid ───────────────────────────────────────────────── */
.main {
  flex: 1;
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 20px;
  padding: 20px 24px 24px;
  align-items: start;
}

/* ── Left column ─────────────────────────────────────────────── */
.left-col { position: sticky; top: 20px; }
.left-card {
  background: var(--card);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  display: flex; flex-direction: column;
  max-height: calc(100dvh - 140px);
  overflow: hidden;
}

.left-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 16px 12px;
  border-bottom: 1px solid var(--line);
}
.panel-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 14px; font-weight: 600; color: #3C38B6;
}
.count-chip {
  background: rgba(59,63,224,.1); color: var(--brand);
  font: 600 12px/1 'Inter', sans-serif;
  padding: 3px 8px; border-radius: 20px;
}

.search-wrap {
  position: relative; padding: 12px 12px 8px;
}
.search-icon {
  position: absolute; left: 22px; top: 50%; transform: translateY(-50%);
  color: var(--ink-soft); pointer-events: none;
}
.search-input {
  width: 100%; height: 36px;
  border: 1.5px solid var(--line); border-radius: 8px;
  background: var(--bg); padding: 0 10px 0 34px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.search-input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.search-input::placeholder { color: #b0b3be; }

.chips {
  display: flex; flex-wrap: wrap; gap: 6px; padding: 0 12px 12px;
}
.chip {
  padding: 4px 11px; height: 28px; border: 1.5px solid var(--line); border-radius: 20px;
  background: #fff; color: var(--ink-soft);
  font: 500 12px/1 'Inter', sans-serif; cursor: pointer;
  transition: border-color .15s, background .15s, color .15s;
}
.chip:hover { border-color: var(--brand); color: var(--brand); }
.chip.active {
  background: rgba(59,63,224,.1); border-color: var(--brand);
  color: var(--brand); font-weight: 600;
}

.retakes-list {
  flex: 1; overflow-y: auto; padding: 4px 8px 12px;
  display: flex; flex-direction: column; gap: 6px;
}
.retakes-list::-webkit-scrollbar { width: 4px; }
.retakes-list::-webkit-scrollbar-track { background: transparent; }
.retakes-list::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }

.r-item {
  padding: 12px 12px;
  border-radius: 8px; border: 1.5px solid var(--line);
  background: var(--bg); cursor: pointer;
  transition: border-color .15s var(--ease), background .15s var(--ease), box-shadow .15s var(--ease);
}
.r-item:hover { border-color: rgba(59,63,224,.3); background: rgba(59,63,224,.03); }
.r-item.selected {
  border-color: var(--brand); background: rgba(59,63,224,.06);
  box-shadow: 0 0 0 3px rgba(59,63,224,.1);
}
.r-subject {
  font-size: 13px; font-weight: 600; color: var(--ink);
  margin-bottom: 4px; line-height: 1.3;
}
.r-meta { font-size: 11px; color: var(--ink-soft); line-height: 1.6; }
.r-footer {
  display: flex; align-items: center; justify-content: space-between;
  margin-top: 8px;
}
.status-badge {
  display: inline-block; padding: 3px 9px;
  border-radius: 20px; font: 600 11px/1 'Inter', sans-serif;
}
.status-badge.lg { font-size: 12px; padding: 4px 12px; }
.r-count {
  display: flex; align-items: center; gap: 4px;
  font: 500 11px/1 'Inter', sans-serif; color: var(--ink-soft);
}

.empty-list {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 8px; padding: 32px 0;
  font-size: 13px; color: #b0b3be; text-align: center;
}

/* ── Inline spinner ──────────────────────────────────────────── */
.spinner-inline {
  width: 22px; height: 22px; border-radius: 50%;
  border: 2.5px solid var(--line); border-top-color: var(--brand);
  animation: stmt-spin .8s linear infinite;
}
@keyframes stmt-spin { to { transform: rotate(360deg) } }

.detail-loading {
  display: flex; align-items: center; gap: 10px;
  padding: 28px 24px; font: 500 13px/1 'Inter', sans-serif; color: var(--ink-soft);
}

/* ── Grade note (banner о режиме) ────────────────────────────── */
.grade-note {
  display: flex; align-items: center; gap: 8px;
  margin: 14px 24px 0; padding: 10px 14px; border-radius: 8px;
  font: 500 12.5px/1.4 'Inter', sans-serif;
  background: rgba(107,114,128,.08); color: var(--ink-soft);
}
.grade-note svg { flex-shrink: 0; }
.grade-note--active { background: rgba(245,158,11,.1); color: #b45309; }

/* ── Right column ────────────────────────────────────────────── */
.right-col { display: flex; flex-direction: column; gap: 16px; }

.empty-state {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 80px 0; gap: 12px;
  background: var(--card); border-radius: var(--radius); box-shadow: var(--shadow);
}
.empty-title { font-size: 16px; font-weight: 600; color: var(--ink); margin: 0; }
.empty-sub { font-size: 13px; color: var(--ink-soft); margin: 0; text-align: center; line-height: 1.6; }

/* ── Info card ───────────────────────────────────────────────── */
.info-card {
  background: var(--card); border-radius: var(--radius); box-shadow: var(--shadow);
  padding: 20px 24px;
}
.info-head {
  display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
  margin-bottom: 16px;
}
.retake-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 17px; font-weight: 700; color: #3C38B6; margin: 0; flex: 1;
}
.info-grid {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 8px 28px; margin-bottom: 16px;
}
.info-row {
  display: flex; align-items: center; gap: 8px;
  font-size: 13px; color: var(--ink-soft); line-height: 1.4;
}
.info-row svg { color: var(--brand); flex-shrink: 0; }
.info-row-full { grid-column: 1 / -1; }
.info-footer {
  display: flex; justify-content: flex-end;
  padding-top: 14px; border-top: 1px solid var(--line);
}

.export-group { display: flex; gap: 8px; }
.btn-exp {
  display: flex; align-items: center; gap: 6px;
  height: 34px; padding: 0 14px; border-radius: 8px;
  font: 600 12px/1 'Inter', sans-serif; cursor: pointer;
  transition: background .15s, border-color .15s;
}
.btn-exp.excel { border: 1.5px solid #1a7340; color: #1a7340; background: #fff; }
.btn-exp.excel:hover { background: rgba(26,115,64,.07); }
.btn-exp.word  { border: 1.5px solid #1a56a0; color: #1a56a0; background: #fff; }
.btn-exp.word:hover  { background: rgba(26,86,160,.07); }

/* ── Students card ───────────────────────────────────────────── */
.students-card {
  background: var(--card); border-radius: var(--radius); box-shadow: var(--shadow);
}
.students-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 24px 14px;
  border-bottom: 1px solid var(--line);
}
.students-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 14px; font-weight: 600; color: #3C38B6; margin: 0;
}
.progress-text { font-size: 13px; color: var(--ink-soft); }
.progress-text strong { color: var(--ink); }

.table-wrap { overflow-x: auto; }
.tbl {
  width: 100%; border-collapse: collapse;
}
.tbl th {
  padding: 10px 16px; text-align: left;
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  background: var(--bg); border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.tbl td {
  padding: 11px 16px;
  font-size: 13px; color: var(--ink);
  border-bottom: 1px solid var(--line);
  vertical-align: middle; text-align: left;
}
.tbl tbody tr:last-child td { border-bottom: none; }
.tbl tbody tr { transition: background .12s; }
.tbl tbody tr:hover { background: rgba(59,63,224,.03); }

.td-num   { text-align: center; color: var(--ink-soft); font-weight: 500; }
.td-name  { font-weight: 500; }
.td-group { text-align: left; font-size: 12px; color: var(--ink-soft); font-weight: 500; }
.td-grade { padding-top: 8px; padding-bottom: 8px; }

/* ── Grade picker ────────────────────────────────────────────── */
.grade-picker { display: flex; gap: 5px; align-items: center; }
.g-btn {
  min-width: 34px; height: 30px; padding: 0 8px;
  border: 1.5px solid var(--line); border-radius: 7px;
  background: #fff; color: var(--ink-soft);
  font: 600 12px/1 'Inter', sans-serif; cursor: pointer;
  transition: border-color .15s, background .15s, color .15s, transform .1s;
}
.g-btn:hover { border-color: currentColor; transform: translateY(-1px); }

.g2  { --c: #dc2626; }
.g3  { --c: #d97706; }
.g4  { --c: #3b82f6; }
.g5  { --c: #059669; }
.g-na { --c: #6b7280; min-width: 42px; }

.g-btn:not(:disabled):hover { color: var(--c); border-color: var(--c); background: color-mix(in srgb, var(--c) 8%, transparent); }
.g-btn:disabled { opacity: .4; cursor: not-allowed; }

.grade-spinner {
  width: 16px; height: 16px; border-radius: 50%;
  border: 2px solid var(--line); border-top-color: var(--brand);
  animation: stmt-spin .7s linear infinite; margin-left: 4px;
}

/* Результат (проставленная оценка / долг) + дата фиксации */
.result-cell { display: flex; flex-direction: column; gap: 2px; }
.result-text { font-size: 15px; font-weight: 600; }
.result-date { font: 11px/1 'Inter', sans-serif; color: var(--ink-soft); }

.empty-cell { text-align: center; color: var(--ink-soft); padding: 28px 16px; }

.grade-error-bar {
  margin: 0; padding: 12px 24px; border-top: 1px solid var(--line);
  font: 500 13px/1.4 'Inter', sans-serif; color: #dc2626;
  background: rgba(220,38,38,.05);
}

/* ── Table footer ────────────────────────────────────────────── */
.table-footer {
  display: flex; align-items: center; justify-content: flex-end; gap: 14px;
  padding: 14px 24px; border-top: 1px solid #d7d9e0;
}
.save-ok {
  display: flex; align-items: center; gap: 6px;
  font: 500 13px/1 'Inter', sans-serif; color: #059669;
}
.btn-primary {
  padding: 0 28px; height: 38px; border: none; border-radius: 10px;
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 6px 18px -6px rgba(91,59,217,.55);
  transition: transform .2s cubic-bezier(.2,.7,.2,1), box-shadow .2s cubic-bezier(.2,.7,.2,1);
}
.btn-primary:hover { transform: scale(1.03); }

/* ── Export panel ────────────────────────────────────────────── */
/* Все значения хардкодированы — CSS-переменные не наследуются через Teleport */
.ep-overlay {
  position: fixed; inset: 0;
  background: rgba(10,12,30,.45); z-index: 199;
  opacity: 0; pointer-events: none;
  transition: opacity .25s cubic-bezier(.2,.7,.2,1);
}
.ep-overlay.active { opacity: 1; pointer-events: all; }

.ep {
  position: fixed; top: 0; right: 0; bottom: 0; width: 360px;
  background: #ffffff; z-index: 200;
  display: flex; flex-direction: column;
  box-shadow: -4px 0 32px rgba(20,22,60,.14);
  transform: translateX(100%);
  transition: transform .3s cubic-bezier(.2,.7,.2,1);
  font-family: 'Inter', system-ui, sans-serif;
  color: #1a1d24;
}
.ep.open { transform: translateX(0); }

.ep-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 20px 20px 16px;
  border-bottom: 1px solid #d7d9e0;
  flex-shrink: 0;
}
.ep-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 15px; font-weight: 700; color: #3C38B6; margin: 0;
}
.ep-close {
  width: 32px; height: 32px; border: none; border-radius: 8px;
  background: #f3f4f7; color: #6b7280; cursor: pointer; display: grid; place-items: center;
  transition: background .15s, color .15s;
}
.ep-close:hover { background: rgba(220,38,38,.08); color: #dc2626; }

.ep-body { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 20px; }

.ep-field { display: flex; flex-direction: column; gap: 8px; }
.ep-label { font: 600 12px/1 'Inter', sans-serif; color: #1a1d24; text-transform: uppercase; letter-spacing: .06em; }

.ep-subjects { display: flex; flex-direction: column; gap: 6px; }
.ep-check {
  display: flex; align-items: center; gap: 9px; cursor: pointer;
  font-size: 13px; color: #1a1d24; user-select: none;
}
.ep-check input[type="checkbox"] {
  width: 16px; height: 16px; accent-color: #3b3fe0; flex-shrink: 0; cursor: pointer;
}
.ep-check-text { line-height: 1.4; }
.ep-hint-gray { font-size: 12px; color: #6b7280; margin: 0; font-style: italic; }

.ep-daterange { display: flex; flex-direction: column; gap: 8px; }
.ep-date-wrap { display: flex; align-items: center; gap: 10px; }
.ep-date-lbl { font: 500 13px/1 'Inter', sans-serif; color: #6b7280; width: 20px; }
.ep-input {
  flex: 1; height: 36px;
  border: 1.5px solid #d7d9e0; border-radius: 8px;
  background: #f3f4f7; padding: 0 10px;
  font: 13px/1 'Inter', sans-serif; color: #1a1d24; outline: none;
  transition: border-color .2s cubic-bezier(.2,.7,.2,1);
}
.ep-input:focus { border-color: #3b3fe0; }

.ep-chips { display: flex; flex-wrap: wrap; gap: 6px; }

/* chips внутри панели экспорта — переопределяем без CSS-переменных */
.ep-chips .chip {
  background: #fff; border-color: #d7d9e0; color: #6b7280;
}
.ep-chips .chip:hover { border-color: #3b3fe0; color: #3b3fe0; }
.ep-chips .chip.active {
  background: rgba(59,63,224,.1); border-color: #3b3fe0; color: #3b3fe0;
}

.ep-count {
  font-size: 13px; color: #6b7280;
  padding: 12px 14px; background: #f3f4f7; border-radius: 8px;
}
.ep-count strong { color: #1a1d24; }

.ep-footer {
  padding: 16px 20px; border-top: 1px solid #d7d9e0;
  display: flex; flex-direction: column; gap: 10px; flex-shrink: 0;
}
.btn-exp-lg {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  height: 42px; border-radius: 10px; border: none;
  font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  transition: opacity .2s, transform .15s;
}
.btn-exp-lg:disabled { opacity: .4; cursor: not-allowed; }
.btn-exp-lg:not(:disabled):hover { transform: scale(1.02); }
.btn-exp-lg.excel { background: #1a7340; color: #fff; }
.btn-exp-lg.word  { background: #1a56a0; color: #fff; }

/* ── Transitions ─────────────────────────────────────────────── */
.fade-enter-active, .fade-leave-active { transition: opacity .3s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* ── Responsive ──────────────────────────────────────────────── */
@media (max-width: 1100px) {
  .main { grid-template-columns: 260px 1fr; }
}
@media (max-width: 860px) {
  .main { grid-template-columns: 1fr; }
  .left-col { position: static; }
  .left-card { max-height: 320px; }
  .ep { width: 100%; }
}
@media (max-width: 600px) {
  .main { padding: 16px; gap: 14px; }
  .page-bar { padding: 16px 16px 0; }
  .info-grid { grid-template-columns: 1fr; }
  .grade-picker { flex-wrap: wrap; }
}
</style>
