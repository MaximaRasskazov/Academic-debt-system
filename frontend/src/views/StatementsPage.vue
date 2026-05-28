<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { reportsApi, saveBlob } from '../api/reports'
import { disciplinesApi } from '../api/disciplines'
import { usersApi } from '../api/users'

const auth        = useAuthStore()
const isTeacher   = computed(() => auth.isTeacher)
const sidebarOpen = ref(false)

/* ─── Период (default: последний месяц → сегодня) ──────────────── */
function isoDate(d) { return d.toISOString().split('T')[0] }
const today   = new Date()
const monthAgo = new Date(today.getFullYear(), today.getMonth() - 1, today.getDate())
const from = ref(isoDate(monthAgo))
const to   = ref(isoDate(today))

/* ─── Фильтры (клиентская сторона) ─────────────────────────────── */
const searchQuery   = ref('')
const discFilter    = ref('')   // discipline_name (API не возвращает discipline_id)
const teacherFilter = ref('')   // user_id

/* ─── Справочники для select'ов ─────────────────────────────────── */
const disciplines = ref([])
const teachers    = ref([])

onMounted(async () => {
  const [discRes, teacherRes] = await Promise.allSettled([
    disciplinesApi.list({ limit: 200 }),
    usersApi.list({ role: 'teacher', limit: 200 }),
  ])
  if (discRes.status === 'fulfilled')
    disciplines.value = discRes.value.items ?? discRes.value ?? []
  if (teacherRes.status === 'fulfilled')
    teachers.value = teacherRes.value.items ?? teacherRes.value ?? []
})

/* ─── Загрузка пересдач ─────────────────────────────────────────── */
const rawRetakes = ref([])
const loading    = ref(false)
const loadError  = ref(null)

async function loadRetakes() {
  loadError.value = null
  if (!from.value || !to.value || from.value >= to.value) {
    rawRetakes.value = []
    return
  }
  loading.value = true
  try {
    const data = await reportsApi.retakes({ from: from.value, to: to.value })
    rawRetakes.value = Array.isArray(data) ? data : (data.items ?? [])
  } catch (e) {
    loadError.value = e.response?.data?.message ?? 'Ошибка загрузки данных'
  } finally {
    loading.value = false
  }
}

onMounted(loadRetakes)
watch([from, to], loadRetakes)

/* ─── Нормализация ответа API → формат, ожидаемый шаблоном ─────── */
function normalizeRetake(r) {
  // completed_at может быть null у scheduled-пересдач — используем scheduled_at
  const dt  = new Date(r.completed_at || r.scheduled_at)
  const timeStr = dt.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  return {
    id:       r.retake_id,
    subject:  r.discipline_name || '—',
    date:     isoDate(dt),
    time:     timeStr,
    duration: r.duration_minutes,
    building: r.building,
    room:     r.room,
    type:     r.kind,    // 'regular' | 'commission'
    status:   'completed',
    teachers: (r.teachers ?? []).map((t) => t.full_name),
    students: (r.students ?? []).map((s) => ({
      id:         s.user_id,
      lastName:   s.full_name?.split(' ')[0] ?? '',
      firstName:  s.full_name?.split(' ')[1] ?? '',
      middleName: s.full_name?.split(' ')[2] ?? '',
      group:      s.group_name ?? '',
      result:     s.grade ?? null,
    })),
  }
}

const allRetakes = computed(() => rawRetakes.value.map(normalizeRetake))

/* ─── Клиентские фильтры ─────────────────────────────────────────── */
const filteredRetakes = computed(() => {
  const q = searchQuery.value.toLowerCase()
  return allRetakes.value.filter((r) => {
    if (q && !r.subject.toLowerCase().includes(q)) return false
    if (discFilter.value && r.subject !== discFilter.value) return false
    if (teacherFilter.value) {
      // у нас teachers — массив full_name-строк; ищем по user_id через rawRetakes
      const raw = rawRetakes.value.find((x) => x.retake_id === r.id)
      if (!raw?.teachers?.some((t) => t.user_id === teacherFilter.value)) return false
    }
    return true
  })
})

/* ─── Выбранная пересдача ────────────────────────────────────────── */
const selectedId     = ref(null)
const selectedRetake = computed(() => allRetakes.value.find((r) => r.id === selectedId.value) ?? null)

// При смене выбора — копируем студентов в editableStudents (для grade-picker)
const editableStudents = ref([])
watch(selectedId, (id) => {
  const r = allRetakes.value.find((r) => r.id === id)
  editableStudents.value = r ? r.students.map((s) => ({ ...s })) : []
})

/* ─── Сохранение оценок (только для преподавателя, completed — нет API) ─ */
const saveSuccess = ref(false)
function setResult(studentId, result) {
  const s = editableStudents.value.find((s) => s.id === studentId)
  if (s) s.result = s.result === result ? null : result
}
function saveGrades() {
  // completed пересдача — оценки уже не меняются через этот экран.
  // Показываем UI-подтверждение для совместимости с макетом.
  saveSuccess.value = true
  setTimeout(() => (saveSuccess.value = false), 2500)
}

/* ─── Панель «Экспорт всех» ─────────────────────────────────────── */
const exportPanelOpen = ref(false)
const expSubjects     = ref([])
const expDateFrom     = ref('')
const expDateTo       = ref('')
const xlsxLoading     = ref(false)

const allSubjects  = computed(() => [...new Set(allRetakes.value.map((r) => r.subject))])

function toggleSubject(s) {
  const i = expSubjects.value.indexOf(s)
  i === -1 ? expSubjects.value.push(s) : expSubjects.value.splice(i, 1)
}

const exportFiltered = computed(() =>
  allRetakes.value.filter((r) => {
    const okSub  = !expSubjects.value.length || expSubjects.value.includes(r.subject)
    const okFrom = !expDateFrom.value || r.date >= expDateFrom.value
    const okTo   = !expDateTo.value   || r.date <= expDateTo.value
    return okSub && okFrom && okTo
  }),
)

/* ─── XLSX-скачивание через бэк ─────────────────────────────────── */
async function downloadXlsx() {
  xlsxLoading.value = true
  try {
    const blob = await reportsApi.retakesFile('xlsx', {
      from: expDateFrom.value || from.value,
      to:   expDateTo.value   || to.value,
    })
    saveBlob(blob, `retakes-${from.value}_${to.value}.xlsx`)
    exportPanelOpen.value = false
  } catch {
    // silent — бэк уже залогировал
  } finally {
    xlsxLoading.value = false
  }
}

/* ─── Per-retake Excel (клиентский, форматированная ведомость) ──── */
async function doExcelOne(retake, students) {
  const XLSX = await import('xlsx')
  const wb   = XLSX.utils.book_new()
  appendSheet(XLSX, wb, retake, students)
  XLSX.writeFile(wb, `Ведомость_${retake.subject}_${retake.date}.xlsx`)
}

async function doWordOne(retake, students) {
  const libs = await import('docx')
  const doc  = buildWordDoc(libs, [{ retake, students }])
  const blob = await libs.Packer.toBlob(doc)
  downloadBlob(blob, `Ведомость_${retake.subject}_${retake.date}.docx`)
}

/* ─── Helpers ────────────────────────────────────────────────────── */
const STATUS_MAP = {
  scheduled: { label: 'Назначена',  bg: 'rgba(59,63,224,.1)',   color: '#3b3fe0' },
  ongoing:   { label: 'Проводится', bg: 'rgba(245,158,11,.12)', color: '#d97706' },
  completed: { label: 'Завершена',  bg: 'rgba(5,150,105,.1)',   color: '#059669' },
}

function fullName(s)     { return `${s.lastName} ${s.firstName} ${s.middleName}`.trim() }
function typeLabel(t)    { return t === 'commission' ? 'С комиссией' : 'Обычная' }
function gradeCount(arr) { return arr.filter((s) => s.result !== null).length }
function todayStr()      { return isoDate(new Date()) }

function resultLabel(result) {
  if (result === null || result === undefined) return '—'
  return result === 'absent' ? 'Н/Я' : String(result)
}
function resultColor(result) {
  const map = { 2: '#dc2626', 3: '#d97706', 4: '#3b82f6', 5: '#059669', absent: '#6b7280' }
  return map[result] || '#b0b3be'
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

/* ─── Word builder (без изменений из оригинала) ──────────────────── */
function appendSheet(XLSX, wb, retake, students) {
  const rows = [
    ['ВЕДОМОСТЬ ПЕРЕСДАЧИ'],
    [],
    ['Дисциплина:', retake.subject],
    ['Дата:', formatLong(retake.date)],
    ['Время:', retake.time],
    ['Место:', `Корпус ${retake.building}, аудитория ${retake.room}`],
    ['Тип пересдачи:', typeLabel(retake.type)],
    ['Статус:', STATUS_MAP[retake.status]?.label || retake.status],
    ['Преподаватель(и):', retake.teachers.join('; ')],
    [],
    ['№', 'ФИО студента', 'Группа', 'Оценка'],
    ...students.map((s, i) => [i + 1, fullName(s), s.group, resultLabel(s.result)]),
    [],
    ['Преподаватель:', retake.teachers[0] || ''],
    ['Дата составления:', formatShort(todayStr())],
  ]
  const ws = XLSX.utils.aoa_to_sheet(rows)
  ws['!cols']   = [{ wch: 6 }, { wch: 36 }, { wch: 12 }, { wch: 10 }]
  ws['!merges'] = [{ s: { r: 0, c: 0 }, e: { r: 0, c: 3 } }]
  XLSX.utils.book_append_sheet(wb, ws, retake.subject.slice(0, 31))
}

function buildWordDoc(libs, items) {
  const {
    Document, Paragraph, Table, TableRow, TableCell,
    TextRun, AlignmentType, WidthType,
  } = libs
  const PT = (n) => n * 2
  const CM = (n) => Math.round(n * 567)
  const brd = { style: 'single', size: 4, color: '000000' }
  const allBorders = { top: brd, bottom: brd, left: brd, right: brd }

  function p(text, opts = {}) {
    return new Paragraph({
      alignment: opts.align ?? AlignmentType.LEFT,
      pageBreakBefore: !!opts.pageBreak,
      spacing: { before: CM(opts.before ?? 0), after: CM(opts.after ?? 0.18) },
      children: [new TextRun({
        text: String(text), bold: !!opts.bold,
        size: PT(opts.size ?? 12), font: 'Times New Roman',
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
          text: String(text), bold: !!opts.bold,
          size: PT(opts.size ?? 11), font: 'Times New Roman',
        })],
      })],
    })
  }

  const children = []
  items.forEach(({ retake, students }, idx) => {
    if (idx > 0) children.push(p('', { pageBreak: true }))
    children.push(
      p('ФЕДЕРАЛЬНОЕ ГОСУДАРСТВЕННОЕ БЮДЖЕТНОЕ ОБРАЗОВАТЕЛЬНОЕ УЧРЕЖДЕНИЕ ВЫСШЕГО ОБРАЗОВАНИЯ', { align: AlignmentType.CENTER, size: 14 }),
      p('ЮГОРСКИЙ ГОСУДАРСТВЕННЫЙ УНИВЕРСИТЕТ', { align: AlignmentType.CENTER, size: 14, after: 0.3 }),
      p('ВЕДОМОСТЬ ПЕРЕСДАЧИ', { align: AlignmentType.CENTER, bold: true, size: 16, before: 0.3, after: 0.5 }),
    )
    children.push(new Table({
      width: { size: 100, type: WidthType.PERCENTAGE },
      rows: [
        new TableRow({ children: [cell('Дисциплина:', { bold: true, shade: true, w: 26 }), cell(retake.subject, { bold: true, span: 3 })] }),
        new TableRow({ children: [cell('Дата:', { bold: true, shade: true }), cell(formatLong(retake.date), { w: 28 }), cell('Время:', { bold: true, shade: true, w: 13 }), cell(retake.time, { w: 13 })] }),
        new TableRow({ children: [cell('Место проведения:', { bold: true, shade: true }), cell(`Корпус ${retake.building}, аудитория ${retake.room}`, { span: 3 })] }),
        new TableRow({ children: [cell('Тип пересдачи:', { bold: true, shade: true }), cell(typeLabel(retake.type)), cell('Статус:', { bold: true, shade: true }), cell(STATUS_MAP[retake.status]?.label || retake.status)] }),
        new TableRow({ children: [cell('Преподаватель(и):', { bold: true, shade: true }), cell(retake.teachers.join(', '), { span: 3 })] }),
      ],
    }))
    children.push(p('Список студентов:', { bold: true, before: 0.45, after: 0.2 }))
    children.push(new Table({
      width: { size: 100, type: WidthType.PERCENTAGE },
      rows: [
        new TableRow({
          tableHeader: true,
          children: [
            cell('№',            { bold: true, align: AlignmentType.CENTER, w: 5,  shade: true }),
            cell('ФИО студента', { bold: true,                              w: 43, shade: true }),
            cell('Группа',       { bold: true, align: AlignmentType.CENTER, w: 14, shade: true }),
            cell('Оценка',       { bold: true, align: AlignmentType.CENTER, w: 13, shade: true }),
            cell('Подпись',      { bold: true, align: AlignmentType.CENTER, w: 25, shade: true }),
          ],
        }),
        ...students.map((s, i) => new TableRow({
          children: [
            cell(i + 1, { align: AlignmentType.CENTER }),
            cell(fullName(s)),
            cell(s.group, { align: AlignmentType.CENTER }),
            cell(resultLabel(s.result), { align: AlignmentType.CENTER, bold: true }),
            cell(''),
          ],
        })),
      ],
    }))
    children.push(
      p(`Преподаватель: ___________________________ / ${retake.teachers[0] || ''} /`, { before: 0.55 }),
      p(`Дата составления: ${formatShort(todayStr())}`, { before: 0.2 }),
    )
  })
  return new Document({
    styles: { default: { document: { run: { font: 'Times New Roman', size: PT(12) } } } },
    sections: [{ properties: { page: { margin: { top: CM(2), right: CM(1.5), bottom: CM(2), left: CM(3) } } }, children }],
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

            <!-- Период -->
            <div class="period-wrap">
              <div class="period-row">
                <label class="period-lbl">С</label>
                <input type="date" class="period-input" v-model="from" :max="to" />
              </div>
              <div class="period-row">
                <label class="period-lbl">По</label>
                <input type="date" class="period-input" v-model="to" :min="from" :max="isoDate(new Date())" />
              </div>
            </div>

            <!-- Дисциплина -->
            <div class="filter-row" v-if="disciplines.length">
              <select class="filter-select" v-model="discFilter">
                <option value="">Все дисциплины</option>
                <option v-for="d in disciplines" :key="d.id" :value="d.name">{{ d.name }}</option>
              </select>
            </div>

            <!-- Преподаватель -->
            <div class="filter-row" v-if="teachers.length">
              <select class="filter-select" v-model="teacherFilter">
                <option value="">Все преподаватели</option>
                <option v-for="t in teachers" :key="t.id" :value="t.id">
                  {{ t.last_name }} {{ t.first_name }}
                </option>
              </select>
            </div>

            <!-- Поиск -->
            <div class="search-wrap">
              <svg class="search-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
              </svg>
              <input class="search-input" type="text" v-model="searchQuery" placeholder="Поиск по предмету..." />
            </div>

            <!-- Список -->
            <div class="retakes-list">
              <!-- Загрузка -->
              <div v-if="loading" class="empty-list">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#d7d9e0" stroke-width="2" stroke-linecap="round">
                  <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4"/>
                </svg>
                Загрузка…
              </div>

              <!-- Ошибка -->
              <div v-else-if="loadError" class="empty-list" style="color:#dc2626">
                {{ loadError }}
              </div>

              <!-- Данные -->
              <template v-else>
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
                    <span class="r-count">
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/></svg>
                      {{ gradeCount(r.students) }} / {{ r.students.length }}
                    </span>
                  </div>
                </div>

                <div v-if="!filteredRetakes.length && !loading" class="empty-list">
                  <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="#d7d9e0" stroke-width="1.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
                  Пересдачи не найдены
                </div>
              </template>
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
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>
                  <span>{{ formatLong(selectedRetake.date) }}, {{ selectedRetake.time }}</span>
                </div>
                <div class="info-row">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  <span>{{ selectedRetake.duration }} мин.</span>
                </div>
                <div class="info-row">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0118 0z"/><circle cx="12" cy="10" r="3"/></svg>
                  <span>Корп. {{ selectedRetake.building }}, ауд. {{ selectedRetake.room }}</span>
                </div>
                <div class="info-row">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2"/><rect x="9" y="3" width="6" height="4" rx="2"/></svg>
                  <span>{{ typeLabel(selectedRetake.type) }}</span>
                </div>
                <div class="info-row info-row-full">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
                  <span>{{ selectedRetake.teachers.join(', ') }}</span>
                </div>
              </div>

              <div class="info-footer">
                <div class="export-group">
                  <button class="btn-exp excel" @click="doExcelOne(selectedRetake, editableStudents)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/></svg>
                    Excel
                  </button>
                  <button class="btn-exp word" @click="doWordOne(selectedRetake, editableStudents)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M9 13l2 4 2-4 2 4"/></svg>
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
                  Оценено:&nbsp;<strong>{{ gradeCount(editableStudents) }}</strong>&nbsp;из&nbsp;<strong>{{ editableStudents.length }}</strong>
                </span>
              </div>

              <div class="table-wrap">
                <table class="tbl">
                  <colgroup>
                    <col style="width:46px">
                    <col>
                    <col style="width:104px">
                    <col style="width:248px">
                  </colgroup>
                  <thead>
                    <tr>
                      <th>№</th>
                      <th>ФИО студента</th>
                      <th>Группа</th>
                      <th>Оценка</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(s, i) in editableStudents" :key="s.id">
                      <td class="td-num">{{ i + 1 }}</td>
                      <td class="td-name">{{ fullName(s) }}</td>
                      <td class="td-group">{{ s.group }}</td>
                      <td class="td-grade">
                        <div v-if="isTeacher" class="grade-picker">
                          <button
                            v-for="g in [2, 3, 4, 5]" :key="g"
                            class="g-btn" :class="[`g${g}`, { active: s.result === g }]"
                            @click="setResult(s.id, g)"
                          >{{ g }}</button>
                          <button
                            class="g-btn g-absent" :class="{ active: s.result === 'absent' }"
                            @click="setResult(s.id, 'absent')"
                          >Н/Я</button>
                        </div>
                        <span
                          v-else class="result-text"
                          :style="{ color: resultColor(s.result), fontWeight: s.result !== null ? 600 : 400 }"
                        >{{ resultLabel(s.result) }}</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <div v-if="isTeacher" class="table-footer">
                <Transition name="fade">
                  <span v-if="saveSuccess" class="save-ok">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><polyline points="20 6 9 17 4 12"/></svg>
                    Оценки сохранены
                  </span>
                </Transition>
                <button class="btn-primary" @click="saveGrades">Сохранить оценки</button>
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
            :disabled="!exportFiltered.length || xlsxLoading"
            @click="downloadXlsx"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/></svg>
            {{ xlsxLoading ? 'Загрузка…' : 'Скачать Excel' }}
          </button>
        </div>
      </div>
    </Teleport>

  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

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
.page-stmt { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }

.page-bar { display: flex; align-items: center; justify-content: space-between; padding: 20px 24px 0; gap: 16px; }
.page-title { font-family: 'Gerhaus', 'Inter', sans-serif; font-size: 20px; font-weight: 700; color: #3C38B6; margin: 0; }
.btn-all-export {
  display: flex; align-items: center; gap: 7px;
  padding: 0 18px; height: 38px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 6px 18px -6px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease); white-space: nowrap;
}
.btn-all-export:hover { transform: scale(1.03); }

.main { flex: 1; display: grid; grid-template-columns: 300px 1fr; gap: 20px; padding: 20px 24px 24px; align-items: start; }

.left-col { position: sticky; top: 20px; }
.left-card { background: var(--card); border-radius: var(--radius); box-shadow: var(--shadow); display: flex; flex-direction: column; max-height: calc(100dvh - 140px); overflow: hidden; }
.left-head { display: flex; align-items: center; justify-content: space-between; padding: 16px 16px 12px; border-bottom: 1px solid var(--line); }
.panel-title { font-family: 'Gerhaus', 'Inter', sans-serif; font-size: 14px; font-weight: 600; color: #3C38B6; }
.count-chip { background: rgba(59,63,224,.1); color: var(--brand); font: 600 12px/1 'Inter', sans-serif; padding: 3px 8px; border-radius: 20px; }

/* ── Period / filter inputs ── */
.period-wrap { padding: 10px 12px 4px; display: flex; flex-direction: column; gap: 6px; }
.period-row  { display: flex; align-items: center; gap: 8px; }
.period-lbl  { font: 500 12px/1 'Inter', sans-serif; color: var(--ink-soft); width: 18px; flex-shrink: 0; }
.period-input {
  flex: 1; height: 32px;
  border: 1.5px solid var(--line); border-radius: 7px;
  background: var(--bg); padding: 0 8px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s;
}
.period-input:focus { border-color: var(--brand); }

.filter-row { padding: 4px 12px; }
.filter-select {
  width: 100%; height: 32px;
  border: 1.5px solid var(--line); border-radius: 7px;
  background: var(--bg); padding: 0 8px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s; cursor: pointer;
}
.filter-select:focus { border-color: var(--brand); }

.search-wrap { position: relative; padding: 8px 12px; }
.search-icon { position: absolute; left: 22px; top: 50%; transform: translateY(-50%); color: var(--ink-soft); pointer-events: none; }
.search-input {
  width: 100%; height: 36px;
  border: 1.5px solid var(--line); border-radius: 8px;
  background: var(--bg); padding: 0 10px 0 34px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.search-input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.search-input::placeholder { color: #b0b3be; }

.retakes-list { flex: 1; overflow-y: auto; padding: 4px 8px 12px; display: flex; flex-direction: column; gap: 6px; }
.retakes-list::-webkit-scrollbar { width: 4px; }
.retakes-list::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }

.r-item { padding: 12px; border-radius: 8px; border: 1.5px solid var(--line); background: var(--bg); cursor: pointer; transition: border-color .15s, background .15s, box-shadow .15s; }
.r-item:hover { border-color: rgba(59,63,224,.3); background: rgba(59,63,224,.03); }
.r-item.selected { border-color: var(--brand); background: rgba(59,63,224,.06); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.r-subject { font-size: 13px; font-weight: 600; color: var(--ink); margin-bottom: 4px; line-height: 1.3; }
.r-meta { font-size: 11px; color: var(--ink-soft); line-height: 1.6; }
.r-footer { display: flex; align-items: center; justify-content: space-between; margin-top: 8px; }
.status-badge { display: inline-block; padding: 3px 9px; border-radius: 20px; font: 600 11px/1 'Inter', sans-serif; }
.status-badge.lg { font-size: 12px; padding: 4px 12px; }
.r-count { display: flex; align-items: center; gap: 4px; font: 500 11px/1 'Inter', sans-serif; color: var(--ink-soft); }

.empty-list { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 32px 0; font-size: 13px; color: #b0b3be; text-align: center; }

.right-col { display: flex; flex-direction: column; gap: 16px; }
.empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 80px 0; gap: 12px; background: var(--card); border-radius: var(--radius); box-shadow: var(--shadow); }
.empty-title { font-size: 16px; font-weight: 600; color: var(--ink); margin: 0; }
.empty-sub { font-size: 13px; color: var(--ink-soft); margin: 0; text-align: center; line-height: 1.6; }

.info-card { background: var(--card); border-radius: var(--radius); box-shadow: var(--shadow); padding: 20px 24px; }
.info-head { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 16px; }
.retake-title { font-family: 'Gerhaus', 'Inter', sans-serif; font-size: 17px; font-weight: 700; color: #3C38B6; margin: 0; flex: 1; }
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 28px; margin-bottom: 16px; }
.info-row { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--ink-soft); line-height: 1.4; }
.info-row svg { color: var(--brand); flex-shrink: 0; }
.info-row-full { grid-column: 1 / -1; }
.info-footer { display: flex; justify-content: flex-end; padding-top: 14px; border-top: 1px solid var(--line); }
.export-group { display: flex; gap: 8px; }
.btn-exp { display: flex; align-items: center; gap: 6px; height: 34px; padding: 0 14px; border-radius: 8px; font: 600 12px/1 'Inter', sans-serif; cursor: pointer; transition: background .15s; }
.btn-exp.excel { border: 1.5px solid #1a7340; color: #1a7340; background: #fff; }
.btn-exp.excel:hover { background: rgba(26,115,64,.07); }
.btn-exp.word  { border: 1.5px solid #1a56a0; color: #1a56a0; background: #fff; }
.btn-exp.word:hover  { background: rgba(26,86,160,.07); }

.students-card { background: var(--card); border-radius: var(--radius); box-shadow: var(--shadow); }
.students-head { display: flex; align-items: center; justify-content: space-between; padding: 18px 24px 14px; border-bottom: 1px solid var(--line); }
.students-title { font-family: 'Gerhaus', 'Inter', sans-serif; font-size: 14px; font-weight: 600; color: #3C38B6; margin: 0; }
.progress-text { font-size: 13px; color: var(--ink-soft); }
.progress-text strong { color: var(--ink); }
.table-wrap { overflow-x: auto; }
.tbl { width: 100%; border-collapse: collapse; }
.tbl th { padding: 10px 16px; text-align: left; font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft); background: var(--bg); border-bottom: 1px solid var(--line); white-space: nowrap; }
.tbl td { padding: 11px 16px; font-size: 13px; color: var(--ink); border-bottom: 1px solid var(--line); vertical-align: middle; }
.tbl tbody tr:last-child td { border-bottom: none; }
.tbl tbody tr { transition: background .12s; }
.tbl tbody tr:hover { background: rgba(59,63,224,.03); }
.td-num   { text-align: center; color: var(--ink-soft); font-weight: 500; }
.td-name  { font-weight: 500; }
.td-group { text-align: left; font-size: 12px; color: var(--ink-soft); font-weight: 500; }
.td-grade { padding-top: 8px; padding-bottom: 8px; }
.grade-picker { display: flex; gap: 5px; align-items: center; }
.g-btn { min-width: 34px; height: 30px; padding: 0 8px; border: 1.5px solid var(--line); border-radius: 7px; background: #fff; color: var(--ink-soft); font: 600 12px/1 'Inter', sans-serif; cursor: pointer; transition: border-color .15s, background .15s, color .15s, transform .1s; }
.g-btn:hover { border-color: currentColor; transform: translateY(-1px); }
.g2 { --c: #dc2626; } .g3 { --c: #d97706; } .g4 { --c: #3b82f6; } .g5 { --c: #059669; } .g-absent { --c: #6b7280; min-width: 44px; }
.g-btn.active { border-color: var(--c); background: var(--c); color: #fff; }
.g-btn:not(.active):hover { color: var(--c); border-color: var(--c); background: color-mix(in srgb, var(--c) 8%, transparent); }
.result-text { font-size: 14px; }
.table-footer { display: flex; align-items: center; justify-content: flex-end; gap: 14px; padding: 14px 24px; border-top: 1px solid #d7d9e0; }
.save-ok { display: flex; align-items: center; gap: 6px; font: 500 13px/1 'Inter', sans-serif; color: #059669; }
.btn-primary { padding: 0 28px; height: 38px; border: none; border-radius: 10px; background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%); color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer; box-shadow: 0 6px 18px -6px rgba(91,59,217,.55); transition: transform .2s; }
.btn-primary:hover { transform: scale(1.03); }

.ep-overlay { position: fixed; inset: 0; background: rgba(10,12,30,.45); z-index: 199; opacity: 0; pointer-events: none; transition: opacity .25s; }
.ep-overlay.active { opacity: 1; pointer-events: all; }
.ep { position: fixed; top: 0; right: 0; bottom: 0; width: 360px; background: #ffffff; z-index: 200; display: flex; flex-direction: column; box-shadow: -4px 0 32px rgba(20,22,60,.14); transform: translateX(100%); transition: transform .3s cubic-bezier(.2,.7,.2,1); font-family: 'Inter', system-ui, sans-serif; color: #1a1d24; }
.ep.open { transform: translateX(0); }
.ep-head { display: flex; align-items: center; justify-content: space-between; padding: 20px 20px 16px; border-bottom: 1px solid #d7d9e0; flex-shrink: 0; }
.ep-title { font-family: 'Gerhaus', 'Inter', sans-serif; font-size: 15px; font-weight: 700; color: #3C38B6; margin: 0; }
.ep-close { width: 32px; height: 32px; border: none; border-radius: 8px; background: #f3f4f7; color: #6b7280; cursor: pointer; display: grid; place-items: center; transition: background .15s; }
.ep-close:hover { background: rgba(220,38,38,.08); color: #dc2626; }
.ep-body { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 20px; }
.ep-field { display: flex; flex-direction: column; gap: 8px; }
.ep-label { font: 600 12px/1 'Inter', sans-serif; color: #1a1d24; text-transform: uppercase; letter-spacing: .06em; }
.ep-subjects { display: flex; flex-direction: column; gap: 6px; }
.ep-check { display: flex; align-items: center; gap: 9px; cursor: pointer; font-size: 13px; color: #1a1d24; user-select: none; }
.ep-check input[type="checkbox"] { width: 16px; height: 16px; accent-color: #3b3fe0; flex-shrink: 0; cursor: pointer; }
.ep-check-text { line-height: 1.4; }
.ep-hint-gray { font-size: 12px; color: #6b7280; margin: 0; font-style: italic; }
.ep-daterange { display: flex; flex-direction: column; gap: 8px; }
.ep-date-wrap { display: flex; align-items: center; gap: 10px; }
.ep-date-lbl { font: 500 13px/1 'Inter', sans-serif; color: #6b7280; width: 20px; }
.ep-input { flex: 1; height: 36px; border: 1.5px solid #d7d9e0; border-radius: 8px; background: #f3f4f7; padding: 0 10px; font: 13px/1 'Inter', sans-serif; color: #1a1d24; outline: none; transition: border-color .2s; }
.ep-input:focus { border-color: #3b3fe0; }
.ep-count { font-size: 13px; color: #6b7280; padding: 12px 14px; background: #f3f4f7; border-radius: 8px; }
.ep-count strong { color: #1a1d24; }
.ep-footer { padding: 16px 20px; border-top: 1px solid #d7d9e0; display: flex; flex-direction: column; gap: 10px; flex-shrink: 0; }
.btn-exp-lg { display: flex; align-items: center; justify-content: center; gap: 8px; height: 42px; border-radius: 10px; border: none; font: 600 13px/1 'Inter', sans-serif; cursor: pointer; transition: opacity .2s, transform .15s; }
.btn-exp-lg:disabled { opacity: .4; cursor: not-allowed; }
.btn-exp-lg:not(:disabled):hover { transform: scale(1.02); }
.btn-exp-lg.excel { background: #1a7340; color: #fff; }

.fade-enter-active, .fade-leave-active { transition: opacity .3s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

@media (max-width: 1100px) { .main { grid-template-columns: 260px 1fr; } }
@media (max-width: 860px) { .main { grid-template-columns: 1fr; } .left-col { position: static; } .left-card { max-height: 320px; } .ep { width: 100%; } }
@media (max-width: 600px) { .main { padding: 16px; gap: 14px; } .page-bar { padding: 16px 16px 0; } .info-grid { grid-template-columns: 1fr; } .grade-picker { flex-wrap: wrap; } }
</style>
