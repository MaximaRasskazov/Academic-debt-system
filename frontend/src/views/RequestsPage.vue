<script setup>
import { ref, computed, onMounted } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { changeRequestsApi } from '../api/changeRequests'
import { teacherRequestsApi } from '../api/teacherRequests'
import { retakeRequestsApi } from '../api/retakeRequests'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { usersApi } from '../api/users'
import { debtsApi } from '../api/debts'

const sidebarOpen = ref(false)

// ── Tabs ──────────────────────────────────────────────────────
// role   — заявки на повышение до teacher
// change — заявки на изменение существующих пересдач
// create — заявки преподавателей на создание новой пересдачи
const activeTab = ref('role')

// ── Shared dictionaries ───────────────────────────────────────
const userMap   = ref({})
const discMap   = ref({})
const retakeMap = ref({})

// ── Tab 1: teacher-role-requests ──────────────────────────────
const roleRequests   = ref([])
const roleLoading    = ref(true)
const roleErr        = ref('')

// ── Tab 2: retake-change-requests ─────────────────────────────
const changeRequests = ref([])
const changeLoading  = ref(true)
const changeErr      = ref('')

// ── Tab 3: retake-requests (создание пересдачи) ───────────────
const createRequests = ref([])
const createLoading  = ref(true)
const createErr      = ref('')

// ── Helpers ───────────────────────────────────────────────────
function userFio(id) {
  if (!id) return '—'
  return userMap.value[id] || `id:${String(id).slice(0, 8)}…`
}
function userEmail(id) {
  if (!id) return ''
  return userMap.value[`${id}__email`] || ''
}
function disciplineName(retakeId) {
  const retake = retakeMap.value[retakeId]
  return retake ? (discMap.value[retake.discipline_id] || 'Дисциплина') : '—'
}
function retakeScheduled(retakeId) {
  const retake = retakeMap.value[retakeId]
  if (!retake?.scheduled_at) return null
  return retake.scheduled_at
}
function fmtDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}
function fmtTime(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}
function fmtDateTime(iso) {
  if (!iso) return '—'
  return `${fmtDate(iso)} ${fmtTime(iso)}`
}

// Сводка по предлагаемым изменениям и снапшот текущего расписания пересдачи.
// На бэке поля requested_changes: scheduled_at | duration_minutes | building | room | notes
function changeFieldRows(req) {
  const proposed = req.requested_changes || {}
  const retake   = retakeMap.value[req.retake_id] || {}

  // Каждая строка — пара "было → стало". Показываем только поля, которые
  // реально меняются (есть в requested_changes).
  const rows = []
  if (proposed.scheduled_at) {
    rows.push({
      label: 'Дата и время',
      from: fmtDateTime(retake.scheduled_at),
      to:   fmtDateTime(proposed.scheduled_at),
    })
  }
  if (proposed.duration_minutes != null) {
    rows.push({
      label: 'Длительность',
      from: retake.duration_minutes ? `${retake.duration_minutes} мин` : '—',
      to:   `${proposed.duration_minutes} мин`,
    })
  }
  if (proposed.building != null) {
    rows.push({ label: 'Корпус',    from: retake.building || '—', to: proposed.building || '—' })
  }
  if (proposed.room != null) {
    rows.push({ label: 'Аудитория', from: retake.room     || '—', to: proposed.room     || '—' })
  }
  if (proposed.notes != null) {
    rows.push({ label: 'Заметки',   from: retake.notes    || '—', to: proposed.notes    || '—' })
  }
  return rows
}

// ── Counts для badge'ей ───────────────────────────────────────
const roleCount   = computed(() => roleRequests.value.length)
const changeCount = computed(() => changeRequests.value.length)
const createCount = computed(() => createRequests.value.length)

// ── Loaders ───────────────────────────────────────────────────
async function loadRoleTab() {
  roleLoading.value = true
  roleErr.value = ''
  try {
    // teacherRequestsApi.listPending() возвращает массив TeacherRequestResponse.
    const list = await teacherRequestsApi.listPending()
    roleRequests.value = Array.isArray(list) ? list : (list?.items ?? [])
  } catch (e) {
    roleErr.value = e.response?.data?.message || e.response?.data?.error || 'Не удалось загрузить заявки'
  } finally {
    roleLoading.value = false
  }
}

async function loadChangeTab() {
  changeLoading.value = true
  changeErr.value = ''
  try {
    const [reqRes, retakesRes, discsRes] = await Promise.allSettled([
      changeRequestsApi.getAll({ limit: 200 }),
      retakesApi.getAll({ limit: 500 }),
      disciplinesApi.getAll({ limit: 500 }),
    ])

    if (discsRes.status === 'fulfilled') {
      (discsRes.value.data.items ?? discsRes.value.data ?? [])
        .forEach((d) => { discMap.value[d.id] = d.name || d.code })
    }
    if (retakesRes.status === 'fulfilled') {
      (retakesRes.value.data.items ?? retakesRes.value.data ?? [])
        .forEach((r) => { retakeMap.value[r.id] = r })
    }
    if (reqRes.status === 'fulfilled') {
      changeRequests.value = reqRes.value.data.items ?? reqRes.value.data ?? []
    } else {
      changeErr.value = 'Не удалось загрузить заявки на изменение'
    }
  } finally {
    changeLoading.value = false
  }
}

async function loadCreateTab() {
  createLoading.value = true
  createErr.value = ''
  try {
    // discipline + disciplines уже могут быть загружены табом 'change',
    // но если пользователь сразу попал на 'create' — догружаем.
    const [reqRes, discsRes] = await Promise.allSettled([
      retakeRequestsApi.getAll({ limit: 200 }),
      disciplinesApi.getAll({ limit: 500 }),
    ])
    if (discsRes.status === 'fulfilled') {
      (discsRes.value.data.items ?? discsRes.value.data ?? [])
        .forEach((d) => { discMap.value[d.id] = d.name || d.code })
    }
    if (reqRes.status === 'fulfilled') {
      createRequests.value = reqRes.value.data.items ?? reqRes.value.data ?? []
    } else {
      createErr.value = 'Не удалось загрузить заявки на пересдачи'
    }
  } finally {
    createLoading.value = false
  }
}

// Форматирование payload заявки на создание для отображения карточки.
// payload — JSONB с полями, которые описал преподаватель.
function createFieldRows(req) {
  const p = req.payload || {}
  const rows = []
  const kindLabel = p.kind === 'commission' ? 'Комиссия' : 'Обычная'
  rows.push({ label: 'Тип', value: kindLabel })
  if (p.scheduled_at) {
    rows.push({ label: 'Дата и время', value: fmtDateTime(p.scheduled_at) })
  }
  if (p.duration_minutes) {
    rows.push({ label: 'Длительность', value: `${p.duration_minutes} мин` })
  }
  if (p.building || p.room) {
    rows.push({ label: 'Место', value: `${p.building || '—'} / ${p.room || '—'}` })
  }
  const studentCount = (p.student_debt_ids || []).length
  const teacherCount = (p.teacher_ids || []).length
  if (teacherCount) {
    rows.push({ label: p.kind === 'commission' ? 'Комиссия' : 'Преподаватель', value: `${teacherCount} чел.` })
  }
  if (studentCount) {
    rows.push({ label: 'Студенты', value: `${studentCount} чел.` })
  }
  if (p.notes) {
    rows.push({ label: 'Заметки', value: p.notes })
  }
  return rows
}

let usersLoaded = false
let usersLoadPromise = null

async function loadUsers() {
  if (usersLoaded) return
  if (usersLoadPromise) return usersLoadPromise
  usersLoadPromise = (async () => {
    try {
      const res = await usersApi.getAll({ limit: 500 })
      const list = res.data.items ?? res.data ?? []
      list.forEach((u) => {
        const fio = [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ') || u.email
        userMap.value[u.id] = fio
        userMap.value[`${u.id}__email`] = u.email
      })
      usersLoaded = true
    } catch { /* ФИО не критично — покажем id-обрезок */ }
  })()
  return usersLoadPromise
}

onMounted(async () => {
  await loadUsers()
  loadRoleTab()
  loadChangeTab()
  loadCreateTab()
})

// ── Approve / Reject: общий confirm-flow ──────────────────────
const approving  = ref(null)

async function approveRole(r) {
  approving.value = r.id
  try {
    await teacherRequestsApi.approve(r.id)
    roleRequests.value = roleRequests.value.filter((x) => x.id !== r.id)
  } catch (e) {
    alert(e.response?.data?.message || e.response?.data?.error || 'Не удалось одобрить заявку')
  } finally { approving.value = null }
}

async function approveChange(r) {
  approving.value = r.id
  try {
    await changeRequestsApi.approve(r.id)
    changeRequests.value = changeRequests.value.filter((x) => x.id !== r.id)
  } catch (e) {
    alert(e.response?.data?.message || e.response?.data?.error || 'Не удалось одобрить заявку')
  } finally { approving.value = null }
}

async function approveCreate(r) {
  approving.value = r.id
  try {
    await retakeRequestsApi.approve(r.id)
    createRequests.value = createRequests.value.filter((x) => x.id !== r.id)
  } catch (e) {
    alert(e.response?.data?.message || e.response?.data?.error || 'Не удалось одобрить заявку')
  } finally { approving.value = null }
}

// ── Reject modal ──────────────────────────────────────────────
// Один модал на оба таба. kind = 'role' | 'change' определяет
// какой API-метод дёргать и какой список фильтровать на success.
const rejectModal  = ref(null)
const rejectKind   = ref(null)
const rejectReason = ref('')
const rejecting    = ref(false)
const rejectErr    = ref('')

function openReject(r, kind) {
  rejectModal.value  = r
  rejectKind.value   = kind
  rejectReason.value = ''
  rejectErr.value    = ''
}
function closeReject() { rejectModal.value = null; rejectKind.value = null }

async function doReject() {
  const reason = rejectReason.value.trim()
  if (!reason) { rejectErr.value = 'Укажите причину отклонения'; return }
  rejecting.value = true; rejectErr.value = ''
  try {
    const id = rejectModal.value.id
    if (rejectKind.value === 'role') {
      await teacherRequestsApi.reject(id, reason)
      roleRequests.value = roleRequests.value.filter((x) => x.id !== id)
    } else if (rejectKind.value === 'create') {
      await retakeRequestsApi.reject(id, reason)
      createRequests.value = createRequests.value.filter((x) => x.id !== id)
    } else {
      await changeRequestsApi.reject(id, reason)
      changeRequests.value = changeRequests.value.filter((x) => x.id !== id)
    }
    closeReject()
  } catch (e) {
    rejectErr.value = e.response?.data?.message || e.response?.data?.error || 'Ошибка'
  } finally { rejecting.value = false }
}

// Подсказка для отклонения — в reject-модалке.
const rejectTitle = computed(() => {
  switch (rejectKind.value) {
    case 'role':   return 'Отклонить заявку на роль'
    case 'create': return 'Отклонить заявку на пересдачу'
    default:       return 'Отклонить заявку на изменение пересдачи'
  }
})

const rejectSubtitle = computed(() => {
  const r = rejectModal.value
  if (!r) return ''
  if (rejectKind.value === 'role') return userFio(r.requested_by)
  if (rejectKind.value === 'create') {
    const dn = discMap.value[r.payload?.discipline_id] || 'Дисциплина'
    return `${dn} · ${userFio(r.requested_by)}`
  }
  return `${disciplineName(r.retake_id)} · ${userFio(r.requested_by)}`
})

// ── Detail modal ──────────────────────────────────────────────
const detailModal = ref(null)
const studentsExpanded = ref(false)

const STUDENTS_PREVIEW = 5

function openDetailRole(r) {
  detailModal.value = {
    kind:    'role',
    title:   userFio(r.requested_by),
    fields:  [
      { label: 'Пользователь', value: userFio(r.requested_by) },
      { label: 'Email',        value: userEmail(r.requested_by) || '—' },
      { label: 'Подана',       value: fmtDate(r.created_at) },
      { label: 'Мотивация',    value: r.reason || '—' },
    ],
    teachers: [],
    students: [],
    _raw: r,
  }
}

async function openDetailChange(r) {
  const retake = retakeMap.value[r.retake_id] || {}
  detailModal.value = {
    kind:        'change',
    title:       disciplineName(r.retake_id),
    fields: [
      { label: 'Преподаватель', value: userFio(r.requested_by) },
      { label: 'Подана',        value: fmtDate(r.created_at) },
      { label: 'Текущее время', value: fmtDateTime(retake.scheduled_at) },
      { label: 'Корпус / ауд.', value: `${retake.building || '—'} / ${retake.room || '—'}` },
    ],
    changes:      changeFieldRows(r),
    reason:       r.reason || '',
    teachers:     [],
    students:     [],
    loadingParts: true,
    _raw: r,
  }
  // Подгружаем участников пересдачи
  try {
    await loadUsers()
    const res = await retakesApi.getParticipants(r.retake_id)
    const parts = res.data.items ?? res.data ?? []
    if (detailModal.value) {
      detailModal.value.teachers = parts
        .filter(p => p.kind === 'teacher' || p.kind === 'commission_member')
        .map(p => userFio(p.user_id))
      detailModal.value.students = parts
        .filter(p => p.kind === 'student')
        .map(p => userFio(p.user_id))
    }
  } catch { /* участники не критичны */ } finally {
    if (detailModal.value) detailModal.value.loadingParts = false
  }
}

async function openDetailCreate(r) {
  const p = r.payload || {}
  detailModal.value = {
    kind:  'create',
    title: discMap.value[p.discipline_id] || 'Дисциплина',
    fields: [
      { label: 'Преподаватель', value: userFio(r.requested_by) },
      { label: 'Тип',           value: p.kind === 'commission' ? 'С комиссией' : 'Обычная' },
      { label: 'Дата и время',  value: p.scheduled_at ? fmtDateTime(p.scheduled_at) : '—' },
      { label: 'Длительность',  value: p.duration_minutes ? `${p.duration_minutes} мин` : '—' },
      { label: 'Место',         value: [p.building && `корп. ${p.building}`, p.room && `ауд. ${p.room}`].filter(Boolean).join(', ') || '—' },
      { label: 'Подана',        value: fmtDate(r.created_at) },
    ],
    reason:       p.reason || p.notes || '',
    teachers:     (p.teacher_ids || []).map(id => userFio(id)),
    students:     [],
    loadingParts: !!(p.student_debt_ids?.length),
    teacherCount: (p.teacher_ids || []).length,
    studentCount: (p.student_debt_ids || []).length,
    _raw: r,
  }
  // Подгружаем имена студентов: для каждого debt_id запрашиваем сам долг
  // чтобы получить student_id, затем ищем ФИО в userMap.
  if (p.student_debt_ids?.length) {
    try {
      await loadUsers()
      const debtResults = await Promise.allSettled(
        p.student_debt_ids.map(dId => debtsApi.getById(dId))
      )
      if (detailModal.value) {
        detailModal.value.students = debtResults
          .filter(r => r.status === 'fulfilled')
          .map(r => userFio((r.value.data ?? r.value).student_id))
          .filter(Boolean)
      }
    } catch { /* не критично */ } finally {
      if (detailModal.value) detailModal.value.loadingParts = false
    }
  }
}

function closeDetail() { detailModal.value = null; studentsExpanded.value = false }
</script>

<template>
  <div class="requests-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-wrap">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">
        <div class="content-wrap">

          <!-- ── Tabs ── -->
          <div class="page-tabs">
            <button class="tab-btn" :class="{ active: activeTab === 'role' }" @click="activeTab = 'role'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <path d="M16 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/>
                <circle cx="8.5" cy="7" r="4"/>
                <line x1="20" y1="8" x2="20" y2="14"/>
                <line x1="23" y1="11" x2="17" y2="11"/>
              </svg>
              Смена роли
              <span v-if="roleCount > 0" class="tab-badge">{{ roleCount }}</span>
            </button>

            <button class="tab-btn" :class="{ active: activeTab === 'change' }" @click="activeTab = 'change'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="23 4 23 10 17 10"/>
                <polyline points="1 20 1 14 7 14"/>
                <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/>
              </svg>
              Изменения пересдач
              <span v-if="changeCount > 0" class="tab-badge">{{ changeCount }}</span>
            </button>

            <button class="tab-btn" :class="{ active: activeTab === 'create' }" @click="activeTab = 'create'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="16"/><line x1="8" y1="12" x2="16" y2="12"/>
              </svg>
              Заявки на пересдачи
              <span v-if="createCount > 0" class="tab-badge">{{ createCount }}</span>
            </button>
          </div>

          <!-- ═══════════════ TAB 1: РОЛЬ ═══════════════ -->
          <div v-if="activeTab === 'role'" class="section-card">
            <div class="table-header">
              <div class="title-row">
                <h2 class="section-title">Заявки на роль преподавателя</h2>
                <span class="count-badge">{{ roleCount }}</span>
              </div>
            </div>

            <div v-if="roleLoading" class="state-center">
              <div class="spinner" /><span>Загрузка…</span>
            </div>
            <div v-else-if="roleErr" class="state-center state-error">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
              </svg>
              {{ roleErr }}
            </div>
            <div v-else-if="roleRequests.length === 0" class="empty-state">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                <path d="M16 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/>
                <circle cx="8.5" cy="7" r="4"/>
              </svg>
              <span>Новых заявок нет</span>
              <p>Когда студенты подадут заявку — она появится здесь</p>
            </div>

            <div v-else class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr class="head-row">
                    <th>№</th>
                    <th>Пользователь</th>
                    <th>Подана</th>
                    <th>Действия</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in roleRequests" :key="r.id">
                    <td class="td-num">{{ i + 1 }}</td>
                    <td class="td-subject">{{ userFio(r.requested_by) }}</td>
                    <td class="td-nowrap td-soft">{{ fmtDate(r.created_at) }}</td>
                    <td>
                      <div class="action-btns">
                        <button class="btn-sm btn-detail" @click="openDetailRole(r)">Подробнее</button>
                        <button class="btn-sm btn-approve-sm"
                                :disabled="approving === r.id"
                                @click="approveRole(r)">
                          {{ approving === r.id ? '…' : 'Одобрить' }}
                        </button>
                        <button class="btn-sm btn-reject-sm"
                                :disabled="approving === r.id"
                                @click="openReject(r, 'role')">
                          Отклонить
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- ═══════════════ TAB 2: ИЗМЕНЕНИЯ ═══════════════ -->
          <div v-if="activeTab === 'change'" class="section-card">
            <div class="table-header">
              <div class="title-row">
                <h2 class="section-title">Заявки на изменение пересдач</h2>
                <span class="count-badge">{{ changeCount }}</span>
              </div>
            </div>

            <div v-if="changeLoading" class="state-center">
              <div class="spinner" /><span>Загрузка…</span>
            </div>
            <div v-else-if="changeErr" class="state-center state-error">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
              </svg>
              {{ changeErr }}
            </div>
            <div v-else-if="changeRequests.length === 0" class="empty-state">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                <polyline points="23 4 23 10 17 10"/>
                <polyline points="1 20 1 14 7 14"/>
                <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/>
              </svg>
              <span>Новых заявок нет</span>
              <p>Когда преподаватели подадут заявку — она появится здесь</p>
            </div>

            <div v-else class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr class="head-row">
                    <th>№</th>
                    <th>Дисциплина</th>
                    <th>Преподаватель</th>
                    <th>Изменения</th>
                    <th>Подана</th>
                    <th>Действия</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in changeRequests" :key="r.id">
                    <td class="td-num">{{ i + 1 }}</td>
                    <td class="td-subject">{{ disciplineName(r.retake_id) }}</td>
                    <td class="td-soft">{{ userFio(r.requested_by) }}</td>
                    <td class="td-diff">
                      <div v-for="row in changeFieldRows(r)" :key="row.label" class="inline-diff">
                        <span class="inline-diff-label">{{ row.label }}:</span>
                        <span class="inline-diff-from">{{ row.from }}</span>
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="inline-arrow">
                          <line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/>
                        </svg>
                        <span class="inline-diff-to">{{ row.to }}</span>
                      </div>
                      <span v-if="changeFieldRows(r).length === 0" class="td-soft">—</span>
                    </td>
                    <td class="td-nowrap td-soft">{{ fmtDate(r.created_at) }}</td>
                    <td>
                      <div class="action-btns">
                        <button class="btn-sm btn-detail" @click="openDetailChange(r)">Подробнее</button>
                        <button class="btn-sm btn-approve-sm"
                                :disabled="approving === r.id"
                                @click="approveChange(r)">
                          {{ approving === r.id ? '…' : 'Одобрить' }}
                        </button>
                        <button class="btn-sm btn-reject-sm"
                                :disabled="approving === r.id"
                                @click="openReject(r, 'change')">
                          Отклонить
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- ═══════════════ TAB 3: ЗАЯВКИ НА ПЕРЕСДАЧИ ═══════════════ -->
          <div v-if="activeTab === 'create'" class="section-card">
            <div class="table-header">
              <div class="title-row">
                <h2 class="section-title">Заявки на создание пересдач</h2>
                <span class="count-badge">{{ createCount }}</span>
              </div>
            </div>

            <div v-if="createLoading" class="state-center">
              <div class="spinner" /><span>Загрузка…</span>
            </div>
            <div v-else-if="createErr" class="state-center state-error">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
              </svg>
              {{ createErr }}
            </div>
            <div v-else-if="createRequests.length === 0" class="empty-state">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="16"/><line x1="8" y1="12" x2="16" y2="12"/>
              </svg>
              <span>Новых заявок нет</span>
              <p>Когда преподаватели подадут заявку на пересдачу — она появится здесь</p>
            </div>

            <div v-else class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr class="head-row">
                    <th>№</th>
                    <th>Дисциплина</th>
                    <th>Преподаватель</th>
                    <th>Дата и время</th>
                    <th>Участники</th>
                    <th>Подана</th>
                    <th>Действия</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in createRequests" :key="r.id">
                    <td class="td-num">{{ i + 1 }}</td>
                    <td class="td-subject">{{ discMap[r.payload?.discipline_id] || '—' }}</td>
                    <td class="td-soft">{{ userFio(r.requested_by) }}</td>
                    <td class="td-nowrap td-soft">{{ r.payload?.scheduled_at ? fmtDateTime(r.payload.scheduled_at) : '—' }}</td>
                    <td class="td-soft td-nowrap">
                      {{ (r.payload?.teacher_ids?.length || 0) }} пр. / {{ (r.payload?.student_debt_ids?.length || 0) }} ст.
                    </td>
                    <td class="td-nowrap td-soft">{{ fmtDate(r.created_at) }}</td>
                    <td>
                      <div class="action-btns">
                        <button class="btn-sm btn-detail" @click="openDetailCreate(r)">Подробнее</button>
                        <button class="btn-sm btn-approve-sm"
                                :disabled="approving === r.id"
                                @click="approveCreate(r)">
                          {{ approving === r.id ? '…' : 'Одобрить' }}
                        </button>
                        <button class="btn-sm btn-reject-sm"
                                :disabled="approving === r.id"
                                @click="openReject(r, 'create')">
                          Отклонить
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
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
              <span class="modal-title">{{ detailModal.title }}</span>
              <button class="modal-close" @click="closeDetail" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
            </div>

            <div class="modal-body">

              <!-- Поля -->
              <div class="detail-info-grid">
                <div v-for="f in detailModal.fields" :key="f.label" class="detail-info-cell">
                  <span class="dil">{{ f.label }}</span>
                  <span class="div">{{ f.value }}</span>
                </div>
              </div>

              <!-- Изменения (только для change) -->
              <template v-if="detailModal.kind === 'change' && detailModal.changes?.length">
                <div class="detail-section-label">Запрашиваемые изменения</div>
                <div class="detail-diff-block">
                  <div v-for="row in detailModal.changes" :key="row.label" class="detail-diff-row">
                    <span class="dil">{{ row.label }}</span>
                    <span class="diff-from">{{ row.from }}</span>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="diff-arrow">
                      <line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/>
                    </svg>
                    <span class="diff-to">{{ row.to }}</span>
                  </div>
                </div>
              </template>

              <!-- Участники (для change и create) -->
              <template v-if="detailModal.kind === 'change' || detailModal.kind === 'create'">
                <div v-if="detailModal.loadingParts" class="parts-loading">
                  <div class="spinner-sm" /><span>Загрузка участников…</span>
                </div>
                <div v-else class="detail-people">
                  <div class="detail-people-col">
                    <span class="dil">Преподаватели</span>
                    <div class="detail-tags">
                      <span v-for="t in detailModal.teachers" :key="t" class="dtag">{{ t }}</span>
                      <span v-if="!detailModal.teachers.length" class="div">—</span>
                    </div>
                  </div>
                  <div class="detail-people-col">
                    <span class="dil">Студенты ({{ detailModal.students.length }})</span>
                    <div class="detail-tags">
                      <template v-if="detailModal.students.length">
                        <span
                          v-for="s in (studentsExpanded ? detailModal.students : detailModal.students.slice(0, STUDENTS_PREVIEW))"
                          :key="s" class="dtag dtag--student"
                        >{{ s }}</span>
                        <button
                          v-if="detailModal.students.length > STUDENTS_PREVIEW"
                          class="dtag-more"
                          @click="studentsExpanded = !studentsExpanded"
                        >
                          {{ studentsExpanded ? 'Скрыть' : `+ ещё ${detailModal.students.length - STUDENTS_PREVIEW}` }}
                        </button>
                      </template>
                      <span v-else class="div">—</span>
                    </div>
                  </div>
                </div>
              </template>

              <!-- Причина/заметки -->
              <div v-if="detailModal.reason" class="detail-reason">
                <span class="dil">{{ detailModal.kind === 'role' ? 'Мотивация' : 'Причина' }}</span>
                <p class="div">{{ detailModal.reason }}</p>
              </div>

            </div>

            <div class="modal-foot">
              <button class="btn-outline" @click="closeDetail">Закрыть</button>
              <template v-if="detailModal.kind === 'role'">
                <button class="btn-reject-confirm" @click="openReject(detailModal._raw, 'role'); closeDetail()">Отклонить</button>
                <button class="btn-approve-confirm" :disabled="approving === detailModal._raw.id" @click="approveRole(detailModal._raw); closeDetail()">Одобрить</button>
              </template>
              <template v-else-if="detailModal.kind === 'change'">
                <button class="btn-reject-confirm" @click="openReject(detailModal._raw, 'change'); closeDetail()">Отклонить</button>
                <button class="btn-approve-confirm" :disabled="approving === detailModal._raw.id" @click="approveChange(detailModal._raw); closeDetail()">Одобрить</button>
              </template>
              <template v-else-if="detailModal.kind === 'create'">
                <button class="btn-reject-confirm" @click="openReject(detailModal._raw, 'create'); closeDetail()">Отклонить</button>
                <button class="btn-approve-confirm" :disabled="approving === detailModal._raw.id" @click="approveCreate(detailModal._raw); closeDetail()">Одобрить</button>
              </template>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ── Reject modal ── -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="rejectModal" class="modal-overlay" @click.self="closeReject">
          <div class="modal">

            <div class="modal-head">
              <span class="modal-title">{{ rejectTitle }}</span>
              <button class="modal-close" @click="closeReject" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
            </div>

            <div class="modal-body">
              <div class="modal-disc">{{ rejectSubtitle }}</div>

              <div class="field">
                <label class="field-label">Причина отклонения <span class="required">*</span></label>
                <textarea
                  class="input textarea"
                  v-model="rejectReason"
                  placeholder="Укажите причину — она будет отправлена пользователю…"
                  rows="4"
                />
              </div>
              <p v-if="rejectErr" class="form-error">{{ rejectErr }}</p>
            </div>

            <div class="modal-foot">
              <button class="btn-outline" @click="closeReject">Отмена</button>
              <button class="btn-reject-confirm" :disabled="rejecting" @click="doReject">
                {{ rejecting ? 'Отклонение…' : 'Отклонить' }}
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

.requests-root {
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

.page-wrap    { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }
.main         { flex: 1; padding: 24px; }
.content-wrap { display: flex; flex-direction: column; gap: 20px; }

/* ── Tabs ── */
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
  min-width: 20px; height: 20px; padding: 0 6px; border-radius: 10px;
  background: rgba(245,158,11,.25); color: #b45309;
  font: 700 11px/1 'Inter', sans-serif;
}
.tab-btn.active .tab-badge { background: rgba(255,255,255,.25); color: #fff; }

/* ── States ── */
.state-center {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 12px; padding: 60px 20px;
  font: 500 14px/1.5 'Inter', sans-serif; color: var(--ink-soft); text-align: center;
}
.state-error { color: #dc2626; }
.state-error svg { width: 32px; height: 32px; }
.spinner {
  width: 28px; height: 28px; border-radius: 50%;
  border: 3px solid var(--line); border-top-color: var(--brand);
  animation: spin .8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg) } }

/* ── Card ── */
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px;
}

.table-header { margin-bottom: 16px; }
.title-row { display: flex; align-items: center; gap: 10px; margin-bottom: 4px; }
.section-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0;
}
.count-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 24px; height: 22px; padding: 0 7px; border-radius: 11px;
  background: rgba(59,63,224,.1); color: var(--brand-ink);
  font: 600 12px/1 'Inter', sans-serif;
}
.subtitle { font: 13px/1.4 'Inter', sans-serif; color: var(--ink-soft); margin: 0; }

/* ── Empty ── */
.empty-state {
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  padding: 48px 20px; color: var(--ink-soft); text-align: center;
}
.empty-state svg { width: 40px; height: 40px; opacity: .3; }
.empty-state span { font: 500 14px/1 'Inter', sans-serif; }
.empty-state p    { font: 13px/1.5 'Inter', sans-serif; color: #9ca3af; margin: 0; }

/* ── Table ── */
.table-wrap { overflow: auto; border: 1px solid var(--line); border-radius: var(--radius); }
.data-table { width: 100%; border-collapse: collapse; font: 13px/1.4 'Inter', sans-serif; }
.head-row th {
  position: sticky; top: 0; z-index: 2;
  background: #f8f9fb; padding: 10px 14px;
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-align: left; white-space: nowrap; border-bottom: 1px solid var(--line);
}
.data-table tbody tr { transition: background .12s; }
.data-table tbody tr:hover { background: rgba(59,63,224,.03); }
.data-table td { padding: 11px 14px; border-bottom: 1px solid var(--line); color: var(--ink); vertical-align: middle; text-align: left; font-family: 'Inter', system-ui, sans-serif; }
.data-table tbody tr:last-child td { border-bottom: none; }
.td-num     { color: var(--ink-soft); width: 36px; font-family: 'Inter', system-ui, sans-serif; }
.td-subject { font-weight: 600; min-width: 140px; }
.td-nowrap  { white-space: nowrap; }
.td-soft    { color: var(--ink-soft); }

/* ── Inline diff (Tab 2 таблица) ── */
.td-diff { min-width: 220px; }
.inline-diff {
  display: flex; align-items: center; gap: 5px; flex-wrap: wrap;
  font: 12px/1.5 'Inter', sans-serif; margin-bottom: 2px;
}
.inline-diff:last-child { margin-bottom: 0; }
.inline-diff-label { color: var(--ink-soft); font-weight: 600; font-size: 11px; white-space: nowrap; }
.inline-diff-from { color: var(--ink-soft); text-decoration: line-through; opacity: .7; white-space: nowrap; }
.inline-arrow { width: 12px; height: 12px; color: var(--brand); flex-shrink: 0; }
.inline-diff-to { color: var(--brand-ink); font-weight: 600; white-space: nowrap; }

/* ── Action buttons ── */
.action-btns { display: flex; gap: 6px; flex-wrap: wrap; }
.btn-sm {
  height: 30px; padding: 0 12px; border-radius: 8px;
  font: 600 12px/1 'Inter', system-ui, sans-serif;
  cursor: pointer; white-space: nowrap; border: 1.5px solid transparent;
  transition: background .15s, border-color .15s, color .15s;
}
.btn-sm:disabled { opacity: .5; cursor: not-allowed; }

.btn-detail {
  background: #fff; border-color: var(--line); color: var(--ink-soft);
}
.btn-detail:hover { border-color: var(--brand); color: var(--brand); background: rgba(59,63,224,.04); }

.btn-approve-sm { background: #fff; border-color: var(--brand); color: var(--brand); }
.btn-approve-sm:hover:not(:disabled) { background: rgba(59,63,224,.06); }

.btn-reject-sm { background: #fff; border-color: var(--line); color: var(--ink-soft); }
.btn-reject-sm:hover:not(:disabled) { border-color: #c0c2cc; color: var(--ink); }

/* ── Tab 3 stub ── */
.todo-stub {
  display: flex; flex-direction: column; align-items: center; gap: 12px;
  padding: 60px 30px; text-align: center; color: var(--ink-soft);
}
.todo-icon { width: 48px; height: 48px; color: var(--ink-soft); opacity: .4; }
.todo-stub h2 {
  margin: 0; font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 16px; font-weight: 600; color: var(--ink);
}
.todo-stub p { font: 13px/1.6 'Inter', sans-serif; color: var(--ink-soft); margin: 0; max-width: 520px; }
.todo-stub code {
  background: var(--bg); padding: 2px 8px; border-radius: 6px;
  font: 600 12px/1 'JetBrains Mono', ui-monospace, monospace; color: var(--brand-ink);
}
.todo-note { color: #9ca3af; font-size: 12px; margin-top: 6px; }
.todo-link { color: var(--brand); text-decoration: underline; }

/* ── Detail modal content ── */
.detail-info-grid {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 0; border: 1px solid var(--line); border-radius: 10px;
  overflow: hidden; margin-bottom: 16px;
}
.detail-info-cell {
  display: flex; flex-direction: column; gap: 4px;
  padding: 12px 14px; border-bottom: 1px solid var(--line);
  border-right: 1px solid var(--line);
}
.detail-info-cell:nth-child(2n) { border-right: none; }
.detail-info-cell:nth-last-child(-n+2) { border-bottom: none; }

.detail-section-label {
  font: 600 11px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-transform: uppercase; letter-spacing: .05em;
  margin-bottom: 8px;
}
.detail-diff-block {
  background: var(--bg); border-radius: 8px; padding: 10px 12px;
  display: flex; flex-direction: column; gap: 8px; margin-bottom: 16px;
}
.detail-diff-row {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
  font: 12px/1.4 'Inter', sans-serif;
}
.diff-from { color: var(--ink-soft); text-decoration: line-through; opacity: .7; }
.diff-arrow { width: 13px; height: 13px; color: var(--brand); flex-shrink: 0; }
.diff-to { color: var(--brand-ink); font-weight: 600; }

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
.detail-people {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 12px; margin-bottom: 16px;
}
.detail-people-col { display: flex; flex-direction: column; gap: 8px; }
.detail-tags { display: flex; flex-wrap: wrap; gap: 6px; }
.dtag {
  display: inline-flex; align-items: center; padding: 4px 10px;
  background: rgba(59,63,224,.08); color: var(--brand-ink);
  border-radius: 20px; font: 500 12px/1.4 'Inter', sans-serif;
}
.dtag--student { background: rgba(16,185,129,.1); color: #065f46; }
.dtag-more {
  display: inline-flex; align-items: center; padding: 4px 10px;
  background: none; border: 1.5px dashed var(--line); border-radius: 20px;
  color: var(--ink-soft); font: 500 12px/1.4 'Inter', sans-serif;
  cursor: pointer; transition: border-color .15s, color .15s;
}
.dtag-more:hover { border-color: var(--brand); color: var(--brand); }

.detail-reason {
  display: flex; flex-direction: column; gap: 6px;
  padding: 12px 14px; background: var(--bg); border-radius: 10px;
}
.detail-reason .div { margin: 0; white-space: pre-wrap; }

.dil {
  font: 600 11px/1 'Inter', sans-serif;
  color: var(--ink-soft); text-transform: uppercase; letter-spacing: .05em;
}
.div { font: 13px/1.5 'Inter', sans-serif; color: var(--ink); }

.btn-approve-confirm {
  padding: 0 20px; height: 40px;
  border: 1.5px solid var(--brand); border-radius: var(--radius);
  background: #fff; color: var(--brand);
  font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  transition: background .15s, color .15s;
}
.btn-approve-confirm:hover:not(:disabled) { background: rgba(59,63,224,.07); }
.btn-approve-confirm:disabled { opacity: .5; cursor: not-allowed; }

/* ── Modal ── */
.modal-overlay {
  position: fixed; inset: 0; z-index: 400;
  background: rgba(10,12,30,.5); backdrop-filter: blur(3px); -webkit-backdrop-filter: blur(3px);
  display: flex; align-items: center; justify-content: center; padding: 20px;
  --bg:        #f3f4f7;
  --card:      #ffffff;
  --ink:       #1a1d24;
  --ink-soft:  #6b7280;
  --line:      #d7d9e0;
  --brand:     #3b3fe0;
  --brand-ink: #2a2e9e;
  --radius:    10px;
  --ease:      cubic-bezier(.2,.7,.2,1);
  font-family: 'Inter', system-ui, sans-serif;
}
.modal {
  background: var(--card); border-radius: 16px; width: 100%; max-width: 440px;
  box-shadow: 0 24px 64px -12px rgba(10,12,30,.3);
  display: flex; flex-direction: column; max-height: 90vh; overflow: hidden;
}
.modal--detail { max-width: 540px; }
.modal-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 20px; border-bottom: 1px solid var(--line);
}
.modal-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6;
}
.modal-close {
  width: 32px; height: 32px; border-radius: 8px;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s;
}
.modal-close:hover { background: var(--bg); }
.modal-close svg { width: 16px; height: 16px; }

.modal-body   { padding: 20px; display: flex; flex-direction: column; gap: 14px; overflow-y: auto; flex: 1; }
.modal-body::-webkit-scrollbar { width: 4px; }
.modal-body::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }
.modal-disc   { font: 600 13px/1.4 'Inter', sans-serif; color: var(--ink); }

.field        { display: flex; flex-direction: column; gap: 6px; }
.field-label  { font: 500 13px/1 'Inter', sans-serif; color: var(--ink); }
.required     { color: #dc2626; }

.input {
  appearance: none; height: 38px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.textarea { height: auto; padding: 10px 12px; resize: vertical; line-height: 1.5; }
.form-error { font: 12px/1.4 'Inter', sans-serif; color: #dc2626; margin: 0; }

.modal-foot {
  display: flex; justify-content: flex-end; gap: 10px;
  padding: 14px 20px; border-top: 1px solid var(--line);
}
.btn-outline {
  padding: 0 18px; height: 40px; background: #fff;
  border: 1.5px solid #c5c7d4; border-radius: var(--radius);
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer;
  transition: border-color .15s, color .15s;
}
.btn-outline:hover { border-color: var(--brand); color: var(--brand); }

.btn-reject-confirm {
  padding: 0 20px; height: 40px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; color: var(--ink-soft);
  font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  transition: border-color .15s, color .15s;
}
.btn-reject-confirm:hover:not(:disabled) { border-color: #c0c2cc; color: var(--ink); }
.btn-reject-confirm:disabled { opacity: .5; cursor: not-allowed; }

/* ── Modal transition ── */
.modal-enter-active { transition: opacity .2s var(--ease); }
.modal-leave-active { transition: opacity .18s var(--ease); }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .modal { transition: transform .25s var(--ease); }
.modal-leave-active  .modal { transition: transform .2s var(--ease); }
.modal-enter-from .modal, .modal-leave-to .modal { transform: scale(.96) translateY(10px); }

@media (max-width: 640px) {
  .main { padding: 16px; }
  .cc-diff-row { grid-template-columns: 1fr; gap: 2px; }
  .cc-diff-arrow { transform: rotate(90deg); }
}
</style>
