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

async function loadUsers() {
  try {
    const res = await usersApi.getAll({ limit: 500 })
    const list = res.data.items ?? res.data ?? []
    list.forEach((u) => {
      const fio = [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ') || u.email
      userMap.value[u.id] = fio
      userMap.value[`${u.id}__email`] = u.email
    })
  } catch { /* ФИО не критично — покажем id-обрезок */ }
}

onMounted(async () => {
  // Юзеры нужны для ФИО в обоих табах — грузим один раз.
  await loadUsers()
  // Параллельно тянем обе вкладки — badge'и должны быть видны сразу,
  // даже если активен только один таб.
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
              <p class="subtitle">Студенты подают заявку на получение роли teacher. После одобрения роль выдаётся автоматически.</p>
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
                    <th>Email</th>
                    <th>Мотивация</th>
                    <th>Подана</th>
                    <th>Действия</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in roleRequests" :key="r.id">
                    <td class="td-num">{{ i + 1 }}</td>
                    <td class="td-subject">{{ userFio(r.requested_by) }}</td>
                    <td class="td-soft">{{ userEmail(r.requested_by) || '—' }}</td>
                    <td class="td-reason">{{ r.reason || '—' }}</td>
                    <td class="td-nowrap td-soft">{{ fmtDate(r.created_at) }}</td>
                    <td>
                      <div class="action-btns">
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
              <p class="subtitle">Преподаватели просят изменить дату/аудиторию/длительность уже назначенной пересдачи.</p>
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

            <div v-else class="change-cards">
              <div v-for="r in changeRequests" :key="r.id" class="change-card">
                <div class="cc-head">
                  <div class="cc-title">
                    <strong>{{ disciplineName(r.retake_id) }}</strong>
                    <span class="cc-author">от {{ userFio(r.requested_by) }}</span>
                  </div>
                  <span class="cc-date">{{ fmtDate(r.created_at) }}</span>
                </div>

                <div class="cc-diff">
                  <div v-for="row in changeFieldRows(r)" :key="row.label" class="cc-diff-row">
                    <span class="cc-diff-label">{{ row.label }}</span>
                    <span class="cc-diff-from">{{ row.from }}</span>
                    <svg class="cc-diff-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                      <line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/>
                    </svg>
                    <span class="cc-diff-to">{{ row.to }}</span>
                  </div>
                  <div v-if="changeFieldRows(r).length === 0" class="cc-diff-empty">
                    Изменения не указаны
                  </div>
                </div>

                <div v-if="r.reason" class="cc-reason">
                  <span class="cc-reason-label">Причина:</span> {{ r.reason }}
                </div>

                <div class="cc-actions">
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
              </div>
            </div>
          </div>

          <!-- ═══════════════ TAB 3: ЗАЯВКИ НА ПЕРЕСДАЧИ ═══════════════ -->
          <div v-if="activeTab === 'create'" class="section-card">
            <div class="table-header">
              <div class="title-row">
                <h2 class="section-title">Заявки на создание пересдач</h2>
                <span class="count-badge">{{ createCount }}</span>
              </div>
              <p class="subtitle">Преподаватели просят организовать новую пересдачу. После одобрения пересдача создаётся автоматически с участниками из заявки.</p>
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

            <div v-else class="change-cards">
              <div v-for="r in createRequests" :key="r.id" class="change-card">
                <div class="cc-head">
                  <div class="cc-title">
                    <strong>{{ discMap[r.payload?.discipline_id] || 'Дисциплина' }}</strong>
                    <span class="cc-author">от {{ userFio(r.requested_by) }}</span>
                  </div>
                  <span class="cc-date">{{ fmtDate(r.created_at) }}</span>
                </div>

                <div class="cc-diff">
                  <div v-for="row in createFieldRows(r)" :key="row.label" class="cc-create-row">
                    <span class="cc-diff-label">{{ row.label }}</span>
                    <span class="cc-diff-to">{{ row.value }}</span>
                  </div>
                </div>

                <div v-if="r.payload?.reason" class="cc-reason">
                  <span class="cc-reason-label">Причина:</span> {{ r.payload.reason }}
                </div>

                <div class="cc-actions">
                  <button class="btn-sm btn-approve-sm"
                          :disabled="approving === r.id"
                          @click="approveCreate(r)">
                    {{ approving === r.id ? '…' : 'Одобрить и создать' }}
                  </button>
                  <button class="btn-sm btn-reject-sm"
                          :disabled="approving === r.id"
                          @click="openReject(r, 'create')">
                    Отклонить
                  </button>
                </div>
              </div>
            </div>
          </div>

        </div>
      </main>
    </div>

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
.content-wrap { max-width: 1200px; display: flex; flex-direction: column; gap: 20px; }

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
.data-table td { padding: 11px 14px; border-bottom: 1px solid var(--line); color: var(--ink); vertical-align: middle; }
.data-table tbody tr:last-child td { border-bottom: none; }
.td-num     { color: var(--ink-soft); width: 36px; }
.td-subject { font-weight: 600; min-width: 140px; }
.td-nowrap  { white-space: nowrap; }
.td-soft    { color: var(--ink-soft); }
.td-reason  { max-width: 280px; font-size: 12px; color: var(--ink-soft); }

/* ── Change-request cards (Tab 2) ── */
.change-cards { display: flex; flex-direction: column; gap: 14px; }
.change-card {
  border: 1px solid var(--line); border-radius: var(--radius);
  padding: 16px 18px; background: #fff;
  display: flex; flex-direction: column; gap: 12px;
}
.cc-head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.cc-title { display: flex; flex-direction: column; gap: 2px; }
.cc-title strong { font: 600 14px/1.3 'Inter', sans-serif; color: var(--ink); }
.cc-author { font: 12px/1 'Inter', sans-serif; color: var(--ink-soft); }
.cc-date { font: 12px/1 'Inter', sans-serif; color: var(--ink-soft); white-space: nowrap; }

.cc-diff { display: flex; flex-direction: column; gap: 6px; background: var(--bg); padding: 10px 12px; border-radius: 8px; }
.cc-diff-row {
  display: grid;
  grid-template-columns: 100px 1fr auto 1fr;
  align-items: center; gap: 10px;
  font: 12px/1.4 'Inter', sans-serif;
}
.cc-diff-label { color: var(--ink-soft); font-weight: 600; text-transform: uppercase; font-size: 11px; letter-spacing: .03em; }
.cc-diff-from { color: var(--ink-soft); text-decoration: line-through; opacity: .7; }
.cc-diff-arrow { width: 14px; height: 14px; color: var(--brand); flex-shrink: 0; }
.cc-diff-to { color: var(--brand-ink); font-weight: 600; }
.cc-diff-empty { color: var(--ink-soft); font: italic 12px/1 'Inter', sans-serif; }

/* Tab 3: одиночное значение (не diff) — label слева, value справа. */
.cc-create-row {
  display: grid; grid-template-columns: 130px 1fr;
  align-items: center; gap: 10px;
  font: 12px/1.4 'Inter', sans-serif;
}

.cc-reason { font: 13px/1.5 'Inter', sans-serif; color: var(--ink); padding: 0 2px; }
.cc-reason-label { color: var(--ink-soft); font-weight: 600; }

.cc-actions { display: flex; gap: 8px; justify-content: flex-end; }

/* ── Action buttons ── */
.action-btns { display: flex; gap: 6px; }
.btn-sm { height: 30px; padding: 0 12px; border-radius: 8px; font: 600 12px/1 'Inter', sans-serif; cursor: pointer; white-space: nowrap; border: none; transition: opacity .15s, transform .1s; }
.btn-sm:disabled { opacity: .5; cursor: not-allowed; }

.btn-approve-sm { background: rgba(16,185,129,.12); color: #065f46; }
.btn-approve-sm:hover:not(:disabled) { background: rgba(16,185,129,.2); }

.btn-reject-sm { background: rgba(220,38,38,.08); color: #b91c1c; }
.btn-reject-sm:hover:not(:disabled) { background: rgba(220,38,38,.15); }

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
  display: flex; flex-direction: column;
}
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

.modal-body   { padding: 20px; display: flex; flex-direction: column; gap: 14px; }
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
  padding: 0 20px; height: 40px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #dc2626, #b91c1c);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 4px 14px -4px rgba(185,28,28,.5);
  transition: transform .15s var(--ease);
}
.btn-reject-confirm:hover:not(:disabled) { transform: scale(1.03); }
.btn-reject-confirm:disabled { opacity: .6; cursor: not-allowed; }

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
