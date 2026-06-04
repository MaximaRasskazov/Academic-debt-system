<script setup>
import { ref, computed, reactive, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { useAuthStore } from '../stores/auth'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { usersApi } from '../api/users'
import { debtsApi } from '../api/debts'
import { changeRequestsApi } from '../api/changeRequests'
import { directoryApi } from '../api/directory'
import { fmtDate, fmtTime, partsInTZ, toUtcISO } from '../utils/datetime'
import VueDatePicker from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'

const auth = useAuthStore()
const route = useRoute()
const sidebarOpen = ref(false)

// ── Data ──────────────────────────────────────────────────
const retakes   = ref([])
const discMap   = ref({})
const loading   = ref(true)
const userNames = ref({}) // id → ФИО
const loadErr  = ref('')

// ── Filters ───────────────────────────────────────────────
const search       = ref('')
// По умолчанию показываем активные (назначенные + идущие). Завершённые и
// отменённые не засоряют список — их видно только по явному выбору фильтра.
const statusFilter = ref('active')

const STATUS_LABELS = {
  scheduled:   'Запланирована',
  in_progress: 'Идёт',
  completed:   'Завершена',
  cancelled:   'Отменена',
}
const KIND_LABELS = { regular: 'Обычная', commission: 'С комиссией' }

// 'active' — псевдостатус: scheduled + in_progress. Стоит первым и выбран
// по умолчанию. Завершённые/отменённые — отдельными кнопками.
const ACTIVE_STATUSES = ['scheduled', 'in_progress']
const CHIPS = [
  { value: 'active',      label: 'Актуальные' },
  { value: 'scheduled',   label: 'Запланированные' },
  { value: 'in_progress', label: 'Идут сейчас' },
  { value: 'completed',   label: 'Завершённые' },
  { value: 'cancelled',   label: 'Отменённые' },
  { value: 'all',         label: 'Все' },
]

function matchesStatusFilter(r) {
  if (statusFilter.value === 'all')    return true
  if (statusFilter.value === 'active') return ACTIVE_STATUSES.includes(r.status)
  return r.status === statusFilter.value
}

const filteredRetakes = computed(() => {
  let list = retakes.value.filter(matchesStatusFilter)
  const q = search.value.trim().toLowerCase()
  if (q) {
    list = list.filter(r => {
      const disc = (discMap.value[r.discipline_id] || '').toLowerCase()
      return disc.includes(q) || (r.building || '').toLowerCase().includes(q) || (r.room || '').toLowerCase().includes(q)
    })
  }
  // Новые сперва: по дате проведения, затем по дате создания (если есть).
  return [...list].sort((a, b) => {
    const ta = new Date(a.scheduled_at || a.created_at || 0).getTime()
    const tb = new Date(b.scheduled_at || b.created_at || 0).getTime()
    return tb - ta
  })
})

const counts = computed(() => {
  const m = {}
  for (const c of CHIPS) {
    if (c.value === 'all')         m[c.value] = retakes.value.length
    else if (c.value === 'active') m[c.value] = retakes.value.filter(r => ACTIVE_STATUSES.includes(r.status)).length
    else                           m[c.value] = retakes.value.filter(r => r.status === c.value).length
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
    // Открываем модалку если в URL передан retake_id
    if (route.query.retake_id) {
      const target = retakes.value.find(r => r.id === route.query.retake_id)
      if (target) openEdit(target)
    }
  } finally {
    loading.value = false
  }
})

// fmtDate / fmtTime импортированы из utils/datetime — форматируют в зоне
// вуза (Ханты, UTC+5), а не в зоне устройства.

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
  duration:    '',      // строка чтобы пустое = "не меняем"
  building:    '',
  room:        '',
  notes:       '',
  reason:      '',
})
// Слоты предлагаемых дат (0..3). Пустой список = «дату не меняем».
// Преподаватель может предложить несколько — декан выберет одну.
const REQ_MAX_SLOTS = 3
const reqChangeSlots = ref([])
function reqAddSlot() {
  if (reqChangeSlots.value.length >= REQ_MAX_SLOTS) return
  reqChangeSlots.value.push({ date: '', hour: '', minute: '' })
}
function reqRemoveSlot(i) { reqChangeSlots.value.splice(i, 1) }
const reqChangeSaving    = ref(false)
const reqChangeError     = ref('')
const reqChangeSuccess   = ref(false)
const reqChangeTeachers  = ref([])
const reqChangeStudents  = ref([])
const reqChangePartsLoading = ref(false)

async function openRequestChange(retake) {
  reqChangeTarget.value = retake
  Object.assign(reqChangeForm, {
    duration: '', building: '', room: '', notes: '', reason: '',
  })
  reqChangeSlots.value = []
  reqChangeError.value = ''
  reqChangeSuccess.value = false
  reqChangeTeachers.value = []
  reqChangeStudents.value = []
  reqChangeOpen.value = true

  if (!auth.isDean) return

  reqChangePartsLoading.value = true
  try {
    const [partsRes, usersRes] = await Promise.allSettled([
      retakesApi.getParticipants(retake.id),
      usersApi.getAll({ limit: 500 }),
    ])

    const nameMap = {}
    if (usersRes.status === 'fulfilled') {
      const list = usersRes.value.data.items ?? usersRes.value.data ?? []
      list.forEach(u => {
        nameMap[u.id] = [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ')
      })
    }

    if (partsRes.status === 'fulfilled') {
      const parts = partsRes.value.data.items ?? partsRes.value.data ?? []
      reqChangeTeachers.value = parts
        .filter(p => p.kind === 'teacher' || p.kind === 'commission_member')
        .map(p => nameMap[p.user_id])
        .filter(Boolean)
      reqChangeStudents.value = parts
        .filter(p => p.kind === 'student')
        .map(p => nameMap[p.user_id])
        .filter(Boolean)
    }
  } catch { /* не критично */ } finally {
    reqChangePartsLoading.value = false
  }
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

  const BUILDING_RE_CH = /^[А-Яа-яA-Za-z0-9\s\-\/\.]{1,20}$/
  const ROOM_RE_CH     = /^[А-Яа-яA-Za-z0-9\s\-\/\.]{1,20}$/

  // Слоты дат: собираем заполненные в proposed_slots. Пустой список —
  // дату не меняем. Если слотов несколько, декан выберет один при одобрении.
  const now = new Date()
  const proposed = []
  for (let i = 0; i < reqChangeSlots.value.length; i++) {
    const sl = reqChangeSlots.value[i]
    if (!sl.date && !sl.hour && !sl.minute) continue // пустой слот пропускаем
    if (!sl.date) { reqChangeError.value = `Укажите дату варианта ${i + 1}`; return }
    const hh = String(sl.hour || '00').padStart(2, '0')
    const mm = String(sl.minute || '00').padStart(2, '0')
    const [d, mo, y] = sl.date.split('.')
    if (!d || !mo || !y) { reqChangeError.value = 'Дата в формате дд.мм.гггг'; return }
    const dt = new Date(`${y}-${mo}-${d}T${hh}:${mm}:00`)
    if (dt <= now) { reqChangeError.value = `Дата и время варианта ${i + 1} должны быть в будущем`; return }
    const iso = dt.toISOString()
    if (proposed.includes(iso)) { reqChangeError.value = 'Варианты дат не должны повторяться'; return }
    proposed.push(iso)
  }
  if (proposed.length === 1) {
    changes.scheduled_at = proposed[0]
  } else if (proposed.length > 1) {
    changes.proposed_slots = proposed
    changes.scheduled_at = proposed[0] // дублируем первый для совместимости
  }

  if (f.duration !== '' && f.duration != null) {
    const d = Number(f.duration)
    if (!Number.isInteger(d) || d <= 0) {
      reqChangeError.value = 'Длительность должна быть положительным числом минут'
      return
    }
    changes.duration_minutes = d
  }

  if (f.building.trim() !== '') {
    if (!BUILDING_RE_CH.test(f.building.trim())) {
      reqChangeError.value = 'Некорректный номер корпуса (только буквы, цифры, до 20 символов)'
      return
    }
    changes.building = f.building.trim()
  }
  if (f.room.trim() !== '') {
    if (!ROOM_RE_CH.test(f.room.trim())) {
      reqChangeError.value = 'Некорректный номер аудитории (только буквы, цифры, до 20 символов)'
      return
    }
    changes.room = f.room.trim()
  }
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
const editForm        = reactive({ building: '', room: '', date: '', hour: '09', minute: '00', duration: 90 })
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

// Снимок исходного состава при открытии — по нему в saveEdit считаем
// диф (кого добавить / кого снять). До «Сохранить» изменения состава
// живут только локально и на бэк не уходят.
const editOriginal = ref({ teacherIds: [], studentIds: [] })

const MIN_STUDENTS = 1
function minTeachers() {
  const t = editTarget.value
  return t?.min_teachers ?? (t?.kind === 'commission' ? 3 : 1)
}

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

// Ручная смена статуса деканом: любой → любой (без ограничений переходов).
// Выбор в пикере меняет статус ТОЛЬКО локально (editStatusSelect); запрос
// на сервер (POST /api/retakes/:id/status) уходит в saveEdit по кнопке
// «Сохранить» — вместе с остальными изменениями пересдачи.
const ALL_STATUSES = ['scheduled', 'in_progress', 'completed', 'cancelled']
const editStatusSelect = ref('')   // выбранный статус (применяется по «Сохранить»)
const statusOpen = ref(false)      // открыт ли кастомный дропдаун статуса

function toggleStatusDropdown() {
  statusOpen.value = !statusOpen.value
}
function pickStatus(s) {
  editStatusSelect.value = s
  statusOpen.value = false
}

const STEP = 5, DMIN = 15, DMAX = 480
function clamp(v, mn, mx) { return Math.max(mn, Math.min(mx, v || mn)) }

async function openEdit(r) {
  editTarget.value = r
  editStatusSelect.value = r.status
  editForm.building = r.building || ''
  editForm.room     = r.room     || ''
  editForm.duration = r.duration_minutes || 90
  if (r.scheduled_at) {
    // Предзаполняем дату/часы/минуты временем вуза, а не зоной устройства.
    const p = partsInTZ(r.scheduled_at)
    editForm.date   = `${p.day}.${p.month}.${p.year}` // формат VueDatePicker dd.MM.yyyy
    editForm.hour   = p.hour
    editForm.minute = p.minute
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

    // Строим карту user_id → имя из загруженных пользователей
    const userNameMap = {}
    if (studRes.status === 'fulfilled')
      for (const u of studRes.value.data.items ?? [])
        userNameMap[u.id] = { name: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ') || u.email, group: u.group_name ?? '' }
    if (teachRes.status === 'fulfilled')
      for (const u of teachRes.value.data.items ?? [])
        userNameMap[u.id] = { name: [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ') || u.email, group: '' }

    if (partRes.status === 'fulfilled') {
      const parts = partRes.value.data ?? []
      // Обогащаем участников именами из userNameMap
      const enrich = (p) => ({ ...p, name: userNameMap[p.user_id]?.name || '—', group: userNameMap[p.user_id]?.group || '' })
      editParticipants.value = {
        teachers: parts.filter(p => p.kind === 'teacher' || p.kind === 'commission_member').map(enrich),
        students: parts.filter(p => p.kind === 'student').map(enrich),
      }
      // Запоминаем исходный состав для дифа при сохранении.
      editOriginal.value = {
        teacherIds: editParticipants.value.teachers.map(p => p.user_id),
        studentIds: editParticipants.value.students.map(p => p.user_id),
      }
    }

    const debtMap = {}
    if (debtsRes.status === 'fulfilled') {
      for (const d of debtsRes.value.data.items ?? []) {
        if (d.discipline_id === r.discipline_id && d.status === 'open') debtMap[d.student_id] = d.id
      }
    }
    // Доступные для добавления храним ПОЛНЫМИ (включая уже зачисленных) —
    // видимость в выпадашке рулит computed-фильтр по editParticipants.
    // Так снятый локально участник снова появляется в списке и его можно
    // вернуть до сохранения.
    if (studRes.status === 'fulfilled') {
      editAllStudents.value = (studRes.value.data.items ?? [])
        .filter(u => debtMap[u.id])
        .map(u => ({ id: u.id, name: userNameMap[u.id]?.name || u.email, group: u.group_name ?? '', debtId: debtMap[u.id] }))
    }
    if (teachRes.status === 'fulfilled') {
      editAllTeachers.value = (teachRes.value.data.items ?? [])
        .map(u => ({ id: u.id, name: userNameMap[u.id]?.name || u.email }))
    }
  } finally {
    editLoadingPart.value = false
  }
}

function closeEdit() { editOpen.value = false; editTarget.value = null; statusOpen.value = false }

function handleModalOutsideClick(e) {
  if (!e.target.closest('.picker-wrap-modal')) {
    editTeacherOpen.value = false
    editStudentOpen.value = false
  }
  if (!e.target.closest('.status-select-wrap')) {
    statusOpen.value = false
  }
}
onMounted(() => document.addEventListener('mousedown', handleModalOutsideClick))
onUnmounted(() => document.removeEventListener('mousedown', handleModalOutsideClick))

function onHourBlur()   { editForm.hour   = String(clamp(parseInt(editForm.hour,   10), 0, 23)).padStart(2, '0') }
function onMinuteBlur() { editForm.minute = String(clamp(parseInt(editForm.minute, 10), 0, 59)).padStart(2, '0') }
function onTimeKey(e)   { e.target.value = e.target.value.replace(/\D/g, '').slice(0, 2) }

// Add/remove работают ТОЛЬКО локально — фактические вызовы API уходят
// в saveEdit по «Сохранить». Здесь же держим клиентскую проверку п.5,
// чтобы не дать оголить состав (бэк продублирует её как authoritative).
function removeParticipant(type, userId) {
  editError.value = ''
  if (type === 'teacher') {
    if (editParticipants.value.teachers.length <= minTeachers()) {
      editError.value = editTarget.value?.kind === 'commission'
        ? 'В комиссии должно остаться не меньше 3 преподавателей'
        : 'Должен остаться хотя бы один преподаватель'
      return
    }
    editParticipants.value.teachers = editParticipants.value.teachers.filter(p => p.user_id !== userId)
  } else {
    if (editParticipants.value.students.length <= MIN_STUDENTS) {
      editError.value = 'В пересдаче должен остаться хотя бы один студент'
      return
    }
    editParticipants.value.students = editParticipants.value.students.filter(p => p.user_id !== userId)
  }
}

function addParticipant(type, user) {
  editError.value = ''
  if (type === 'teacher') {
    editParticipants.value.teachers.push({ user_id: user.id, name: user.name, last_name: user.name, kind: 'teacher' })
    editTeacherSearch.value = ''
    editTeacherOpen.value = false
  } else {
    editParticipants.value.students.push({ user_id: user.id, name: user.name, last_name: user.name, group: user.group, kind: 'student', debtId: user.debtId })
    editStudentSearch.value = ''
    editStudentOpen.value = false
  }
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
    const id = editTarget.value.id

    // 1) Диф состава относительно снимка при открытии.
    const origT = new Set(editOriginal.value.teacherIds)
    const origS = new Set(editOriginal.value.studentIds)
    const curT  = editParticipants.value.teachers
    const curS  = editParticipants.value.students
    const curTids = new Set(curT.map(p => p.user_id))
    const curSids = new Set(curS.map(p => p.user_id))

    const teachersToAdd    = curT.filter(p => !origT.has(p.user_id))
    const studentsToAdd    = curS.filter(p => !origS.has(p.user_id))
    const teachersToRemove = [...origT].filter(uid => !curTids.has(uid))
    const studentsToRemove = [...origS].filter(uid => !curSids.has(uid))

    // 2) Сначала добавляем, потом удаляем — чтобы при замене (снять одного,
    //    добавить другого) не нарушить минимум преподавателей на полпути.
    for (const t of teachersToAdd) await retakesApi.addTeacher(id, t.user_id)
    for (const s of studentsToAdd) await retakesApi.addStudent(id, { student_id: s.user_id, debt_id: s.debtId })
    for (const uid of teachersToRemove) await retakesApi.removeTeacher(id, uid)
    for (const uid of studentsToRemove) await retakesApi.removeStudent(id, uid)

    // 3) Расписание шлём только если оно реально изменилось — иначе лишний
    //    апдейт (бэк всё равно не разошлёт уведомления, но не плодим записи).
    const p = partsInTZ(editTarget.value.scheduled_at)
    // Дата редактируется пользователем (формат dd.MM.yyyy). Если поле
    // пустое — оставляем исходную дату пересдачи.
    let dateForIso = `${p.year}-${p.month}-${p.day}`
    let dateChanged = false
    if (editForm.date) {
      const [dd, mm, yy] = editForm.date.split('.')
      if (dd && mm && yy) {
        dateForIso = `${yy}-${mm}-${dd}`
        dateChanged = dd !== p.day || mm !== p.month || yy !== p.year
      }
    }
    const newScheduledAt = toUtcISO(
      dateForIso,
      `${editForm.hour}:${editForm.minute}`,
    )
    const scheduleChanged =
      (editForm.building || '') !== (editTarget.value.building || '') ||
      (editForm.room || '')     !== (editTarget.value.room || '') ||
      Number(editForm.duration) !== Number(editTarget.value.duration_minutes) ||
      dateChanged ||
      editForm.hour   !== p.hour ||
      editForm.minute !== p.minute

    if (scheduleChanged) {
      await retakesApi.update(id, {
        building:         editForm.building,
        room:             editForm.room,
        duration_minutes: editForm.duration,
        scheduled_at:     newScheduledAt,
      })
      const idx = retakes.value.findIndex(r => r.id === id)
      if (idx !== -1) {
        retakes.value[idx] = {
          ...retakes.value[idx],
          building:         editForm.building,
          room:             editForm.room,
          duration_minutes: editForm.duration,
          scheduled_at:     newScheduledAt,
        }
      }
    }

    // 4) Статус: шлём только если декан реально его поменял в пикере.
    if (auth.isDean && editStatusSelect.value && editStatusSelect.value !== editTarget.value.status) {
      await retakesApi.setStatus(id, editStatusSelect.value)
      const idx = retakes.value.findIndex(r => r.id === id)
      if (idx !== -1) retakes.value[idx] = { ...retakes.value[idx], status: editStatusSelect.value }
    }

    closeEdit()
  } catch (e) {
    // Бэк-guard п.5 (последний студент / минимум преподавателей) → 409.
    const code = e?.response?.data?.error
    if (code === 'last_student')      editError.value = 'В пересдаче должен остаться хотя бы один студент'
    else if (code === 'min_teachers') editError.value = 'Преподавателей не может быть меньше минимально допустимого'
    else editError.value = 'Ошибка при сохранении'
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
                <!-- Teacher: запросить изменения — только для запланированных -->
                <button v-if="auth.isTeacher && r.status === 'scheduled'"
                        class="btn-action" @click="openRequestChange(r)">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                    <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                    <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                  </svg>
                  Запросить изменения
                </button>
                <!-- Dean: edit — доступно для любой пересдачи (декан может
                     вручную поправить статус/расписание, в т.ч. откатить
                     ошибочно завершённую) -->
                <button v-if="auth.isDean"
                        class="btn-action" @click="openEdit(r)">
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

            <div class="modal-head modal-head--close-only">
              <button class="modal-close" @click="closeEdit" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
            </div>

            <div class="modal-body">
              <div class="modal-disc">{{ discMap[editTarget?.discipline_id] || 'Дисциплина' }}</div>

              <!-- Статус: декан выставляет вручную любой статус (кастомный дропдаун) -->
              <div class="modal-status-row">
                <template v-if="auth.isDean">
                  <label class="status-select-label">Статус</label>
                  <div class="status-select-wrap" :class="{ open: statusOpen }">
                    <button type="button"
                            class="status-select-trigger"
                            :class="'status-sel-' + editStatusSelect"
                            @click="toggleStatusDropdown">
                      <span>{{ STATUS_LABELS[editStatusSelect] || '—' }}</span>
                      <svg class="status-select-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><polyline points="6 9 12 15 18 9"/></svg>
                    </button>
                    <div v-if="statusOpen" class="status-select-dropdown">
                      <button v-for="s in ALL_STATUSES" :key="s"
                              type="button" class="status-select-option"
                              :class="{ selected: editStatusSelect === s }"
                              @click="pickStatus(s)">
                        <span class="status-dot" :class="'dot-' + s" />
                        {{ STATUS_LABELS[s] }}
                      </button>
                    </div>
                  </div>
                </template>
                <span v-else class="status-badge" :class="'status-' + editTarget?.status">
                  {{ STATUS_LABELS[editTarget?.status] }}
                </span>
              </div>

              <!-- Дата + Время -->
              <div class="form-row">
                <div class="field">
                  <label>Дата</label>
                  <VueDatePicker v-model="editForm.date" locale="ru"
                                 format="dd.MM.yyyy" model-type="format"
                                 :enable-time-picker="false" auto-apply />
                </div>
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
              </div>

              <!-- Длительность -->
              <div class="form-row">
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
              <button class="btn-cancel" type="button" @click="closeEdit">Отмена</button>
              <button class="btn-save" type="button" :disabled="editSaving" @click="saveEdit">
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
          <div class="modal modal--wide">

            <div class="modal-head">
              <span class="modal-title">Запросить изменения пересдачи</span>
              <button class="modal-close" @click="closeRequestChange" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
              </button>
            </div>

            <div class="modal-body modal-body--form">

              <!-- Текущие значения -->
              <div v-if="reqChangeTarget" class="current-info">
                <div class="ci-row">
                  <span class="ci-key">Дата и время</span>
                  <span class="ci-val">{{ fmtDate(reqChangeTarget.scheduled_at) }} в {{ fmtTime(reqChangeTarget.scheduled_at) }}</span>
                </div>
                <div class="ci-row">
                  <span class="ci-key">Место</span>
                  <span class="ci-val">корп. {{ reqChangeTarget.building || '—' }}, ауд. {{ reqChangeTarget.room || '—' }}</span>
                </div>
                <div class="ci-row">
                  <span class="ci-key">Длительность</span>
                  <span class="ci-val">{{ reqChangeTarget.duration_minutes }} мин</span>
                </div>
              </div>

              <!-- Участники (только просмотр) — доступно только декану -->
              <div v-if="auth.isDean" class="req-participants">
                <div class="req-parts-col">
                  <span class="req-parts-label">Преподаватели</span>
                  <div v-if="reqChangePartsLoading" class="req-parts-loading">
                    <div class="spinner-sm" />
                  </div>
                  <div v-else class="req-parts-tags">
                    <span v-for="t in reqChangeTeachers" :key="t" class="rptag">{{ t }}</span>
                    <span v-if="!reqChangeTeachers.length" class="req-parts-empty">—</span>
                  </div>
                </div>
                <div class="req-parts-col">
                  <span class="req-parts-label">Студенты</span>
                  <div v-if="reqChangePartsLoading" class="req-parts-loading">
                    <div class="spinner-sm" />
                  </div>
                  <div v-else class="req-parts-tags">
                    <span v-for="s in reqChangeStudents" :key="s" class="rptag rptag--student">{{ s }}</span>
                    <span v-if="!reqChangeStudents.length" class="req-parts-empty">—</span>
                  </div>
                </div>
              </div>

              <form class="req-form" @submit.prevent="submitRequestChange" novalidate>
                <!-- Новые даты: 0..3 предложенных вариантов. Пусто = дату не меняем. -->
                <div v-for="(sl, i) in reqChangeSlots" :key="i" class="form-row">
                  <div class="field" style="flex:2">
                    <label>{{ i === 0 ? 'Новая дата' : `Новая дата (вариант ${i + 1})` }}</label>
                    <VueDatePicker v-model="sl.date" locale="ru"
                                   format="dd.MM.yyyy" model-type="format"
                                   :enable-time-picker="false" auto-apply />
                  </div>
                  <div class="field">
                    <label>Новое время</label>
                    <div class="time-picker">
                      <input class="time-input" type="text" inputmode="numeric"
                             v-model="sl.hour" maxlength="2"
                             @input="(e) => e.target.value = e.target.value.replace(/\D/g, '').slice(0,2)" />
                      <span class="time-colon">:</span>
                      <input class="time-input" type="text" inputmode="numeric"
                             v-model="sl.minute" maxlength="2"
                             @input="(e) => e.target.value = e.target.value.replace(/\D/g, '').slice(0,2)" />
                    </div>
                  </div>
                  <div class="field field--shrink" style="justify-content:flex-end">
                    <button type="button" class="btn-slot-remove" @click="reqRemoveSlot(i)" title="Удалить вариант">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg>
                    </button>
                  </div>
                </div>
                <div class="slot-add-row">
                  <button type="button" class="btn-add-slot" :disabled="reqChangeSlots.length >= REQ_MAX_SLOTS" @click="reqAddSlot">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                    {{ reqChangeSlots.length ? 'Добавить ещё дату' : 'Предложить новую дату' }}
                  </button>
                  <span class="slot-hint">
                    {{ reqChangeSlots.length >= REQ_MAX_SLOTS ? `Максимум ${REQ_MAX_SLOTS} варианта` : 'Можно предложить несколько — декан выберет одну' }}
                  </span>
                </div>

                <div class="form-row">
                  <div class="field">
                    <label>Длительность (мин)</label>
                    <div class="stepper">
                      <button type="button" class="stepper-btn"
                              @click="reqChangeForm.duration = Math.max(15, (Number(reqChangeForm.duration) || 90) - 5)"
                              :disabled="(Number(reqChangeForm.duration) || 90) <= 15">−</button>
                      <input class="stepper-input" type="number"
                             :value="reqChangeForm.duration || ''"
                             @input="reqChangeForm.duration = $event.target.value"
                             min="15" max="480" />
                      <button type="button" class="stepper-btn"
                              @click="reqChangeForm.duration = Math.min(480, (Number(reqChangeForm.duration) || 90) + 5)"
                              :disabled="(Number(reqChangeForm.duration) || 90) >= 480">+</button>
                    </div>
                  </div>
                  <div class="field">
                    <label>Корпус</label>
                    <input class="input" v-model="reqChangeForm.building" />
                  </div>
                  <div class="field">
                    <label>Аудитория</label>
                    <input class="input" v-model="reqChangeForm.room" />
                  </div>
                </div>

                <div class="form-row">
                  <div class="field field--full">
                    <label>Причина изменения</label>
                    <textarea class="input textarea" rows="3" v-model="reqChangeForm.reason" />
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
  font-family: 'Gerhaus', 'Inter', sans-serif;
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
:global(.modal--wide) { max-width: 720px;
}

.modal-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 20px; border-bottom: 1px solid var(--line); flex-shrink: 0;
}
.modal-head--close-only {
  justify-content: flex-end;
  padding: 12px 14px;
  border-bottom: none;
}
.modal-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
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

.modal-body { flex: 1; overflow-y: auto; padding: 24px; display: flex; flex-direction: column; gap: 20px; }
.modal-body::-webkit-scrollbar { width: 4px; }
.modal-body::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }

.modal-disc {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 17px; font-weight: 700; color: #3C38B6;
  padding: 0 0 4px;
}

.modal-status-row { display: flex; align-items: center; gap: 10px; }
.status-badge {
  display: inline-flex; align-items: center; padding: 4px 12px;
  border-radius: 20px; font: 600 12px/1.4 'Inter', sans-serif;
}
.status-select-label { font: 500 13px/1 'Inter', sans-serif; color: var(--ink); }

/* Кастомный дропдаун статуса (нативный <select> заменён, чтобы не было
   двойной стрелки и неоформленного выпадающего списка). */
.status-select-wrap { position: relative; }
.status-select-trigger {
  display: inline-flex; align-items: center; justify-content: space-between; gap: 8px;
  height: 34px; min-width: 150px; padding: 0 12px;
  border: 1.5px solid var(--line); border-radius: 20px;
  background: var(--card); color: var(--ink);
  font: 600 12px/1 'Inter', sans-serif; cursor: pointer; outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.status-select-trigger:hover:not(:disabled) { border-color: #a0a3b1; }
.status-select-wrap.open .status-select-trigger { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.status-select-trigger:disabled { opacity: .6; cursor: default; }
.status-select-arrow { width: 14px; height: 14px; color: var(--ink-soft); flex-shrink: 0; transition: transform .2s var(--ease); }
.status-select-wrap.open .status-select-arrow { transform: rotate(180deg); }
/* Лёгкая подсветка триггера под текущий статус. */
.status-sel-scheduled   { background-color: rgba(59,63,224,.08); }
.status-sel-in_progress { background-color: rgba(245,158,11,.1); }
.status-sel-completed   { background-color: rgba(16,185,129,.1); }
.status-sel-cancelled   { background-color: rgba(220,38,38,.08); }

.status-select-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; min-width: 180px; z-index: 100;
  background: #fff; border: 1.5px solid var(--line); border-radius: 10px;
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.14); padding: 4px;
}
.status-select-option {
  display: flex; align-items: center; gap: 8px; width: 100%; text-align: left;
  padding: 9px 12px; border: none; border-radius: 6px; background: none;
  font: 500 13px/1.3 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s; white-space: nowrap;
}
.status-select-option:hover { background: rgba(59,63,224,.07); }
.status-select-option.selected { color: var(--brand); font-weight: 600; background: rgba(59,63,224,.06); }
.status-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot-scheduled   { background: #3b3fe0; }
.dot-in_progress { background: #f59e0b; }
.dot-completed   { background: #10b981; }
.dot-cancelled   { background: #9ca3af; }
.status-saving { font: 500 12px/1 'Inter', sans-serif; color: var(--ink-soft); }
.status-scheduled  { background: rgba(59,63,224,.1);  color: #3b3fe0; }
.status-in_progress { background: rgba(245,158,11,.12); color: #b45309; }
.status-completed  { background: rgba(16,185,129,.12); color: #065f46; }
.status-cancelled  { background: rgba(220,38,38,.1);   color: #b91c1c; }
.form-row { display: flex; gap: 16px; }
.field    { flex: 1; display: flex; flex-direction: column; gap: 8px; }
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

/* ── Date slots (мульти-даты) ── */
.btn-slot-remove {
  height: 40px; width: 44px; flex-shrink: 0;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; color: var(--ink-soft); cursor: pointer;
  display: grid; place-items: center;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-slot-remove:hover { border-color: #dc2626; color: #dc2626; background: rgba(220,38,38,.05); }
.btn-slot-remove svg { width: 16px; height: 16px; }
.slot-add-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
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

.req-participants {
  display: grid; grid-template-columns: 1fr 1fr; gap: 16px;
  background: var(--bg); border-radius: var(--radius); padding: 14px 16px;
}
.req-parts-col { display: flex; flex-direction: column; gap: 8px; }
.req-parts-label {
  font: 600 11px/1 'Inter', sans-serif;
  color: var(--ink-soft); text-transform: uppercase; letter-spacing: .05em;
}
.req-parts-loading { display: flex; align-items: center; height: 24px; }
.req-parts-tags { display: flex; flex-wrap: wrap; gap: 6px; }
.req-parts-empty { font: 13px/1 'Inter', sans-serif; color: var(--ink-soft); }
.rptag {
  display: inline-flex; align-items: center; padding: 4px 10px;
  background: rgba(59,63,224,.08); color: var(--brand-ink);
  border-radius: 20px; font: 500 12px/1.4 'Inter', sans-serif;
}
.rptag--student { background: rgba(16,185,129,.1); color: #065f46; }
.spinner-sm {
  width: 16px; height: 16px; border-radius: 50%;
  border: 2px solid var(--line); border-top-color: var(--brand);
  animation: spin .8s linear infinite;
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
