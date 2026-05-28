<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { teacherRequestsApi, changeRequestsApi } from '../api/teacherRequests'

const sidebarOpen = ref(false)
const activeTab = ref('roles')

// ── Loading state ──────────────────────────────────────────
const loading = ref(false)
const loadError = ref('')

// ── Teacher role requests (tab: roles) ────────────────────
const roleRequests = ref([])

async function loadRoleRequests() {
  loading.value = true
  loadError.value = ''
  try {
    const resp = await teacherRequestsApi.listPending()
    roleRequests.value = resp.items || resp || []
  } catch (e) {
    loadError.value = e.response?.data?.message || 'Не удалось загрузить заявки на смену роли'
  } finally {
    loading.value = false
  }
}

// ── Retake change requests (tab: changes) ─────────────────
const changeRequests = ref([])

async function loadChangeRequests() {
  loading.value = true
  loadError.value = ''
  try {
    const resp = await changeRequestsApi.listPending()
    changeRequests.value = resp.items || resp || []
  } catch (e) {
    loadError.value = e.response?.data?.message || 'Не удалось загрузить заявки на изменение пересдач'
  } finally {
    loading.value = false
  }
}

async function refresh() {
  if (activeTab.value === 'roles') await loadRoleRequests()
  else if (activeTab.value === 'changes') await loadChangeRequests()
}

onMounted(() => {
  loadRoleRequests()
  loadChangeRequests()
})

// ── Helpers ────────────────────────────────────────────────
const STATUS_MAP = {
  pending:  { label: 'Ожидает',   color: '#d97706', bg: 'rgba(245,158,11,.12)'  },
  approved: { label: 'Одобрено',  color: '#059669', bg: 'rgba(16,185,129,.12)'  },
  rejected: { label: 'Отклонено', color: '#dc2626', bg: 'rgba(239,68,68,.12)'   },
}

function fmt(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: '2-digit' }) + ' ' +
    d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

function fullName(first, last, middle) {
  return [last, first, middle].filter(Boolean).join(' ')
}

function resetFilters() {
  roleF.user = ''; roleF.status = 'all'
  changeF.retake = ''; changeF.user = ''; changeF.status = 'all'
}

// ── Column filters: roles ──────────────────────────────────
const roleF = reactive({ user: '', status: 'all' })

const filteredRoles = computed(() =>
  roleRequests.value.filter(r => {
    const name = fullName(r.user_first_name, r.user_last_name, r.user_middle_name).toLowerCase()
    if (roleF.user && !name.includes(roleF.user.toLowerCase()) &&
        !(r.user_email || '').toLowerCase().includes(roleF.user.toLowerCase())) return false
    if (roleF.status !== 'all' && r.status !== roleF.status) return false
    return true
  })
)

const pendingRolesCount = computed(() => roleRequests.value.filter(r => r.status === 'pending').length)

// ── Column filters: change requests ───────────────────────
const changeF = reactive({ retake: '', user: '', status: 'all' })

const filteredChanges = computed(() =>
  changeRequests.value.filter(r => {
    if (changeF.retake && !r.retake_id.includes(changeF.retake)) return false
    const name = fullName(r.requester_first_name, r.requester_last_name, r.requester_middle_name).toLowerCase()
    if (changeF.user && !name.includes(changeF.user.toLowerCase()) &&
        !(r.requester_email || '').toLowerCase().includes(changeF.user.toLowerCase())) return false
    if (changeF.status !== 'all' && r.status !== changeF.status) return false
    return true
  })
)

const pendingChangesCount = computed(() => changeRequests.value.filter(r => r.status === 'pending').length)

// ── Reject modal ───────────────────────────────────────────
const rejectModal = reactive({ open: false, req: null, reason: '', type: 'role', submitting: false, error: '' })

function openReject(req, type) {
  rejectModal.req = req
  rejectModal.reason = ''
  rejectModal.type = type
  rejectModal.error = ''
  rejectModal.open = true
}

async function confirmReject() {
  rejectModal.error = ''
  rejectModal.submitting = true
  try {
    if (rejectModal.type === 'role') {
      await teacherRequestsApi.reject(rejectModal.req.id, rejectModal.reason)
      const found = roleRequests.value.find(r => r.id === rejectModal.req.id)
      if (found) found.status = 'rejected'
    } else {
      await changeRequestsApi.reject(rejectModal.req.id, rejectModal.reason)
      const found = changeRequests.value.find(r => r.id === rejectModal.req.id)
      if (found) found.status = 'rejected'
    }
    rejectModal.open = false
  } catch (e) {
    rejectModal.error = e.response?.data?.message || 'Ошибка при отклонении'
  } finally {
    rejectModal.submitting = false
  }
}

// ── Approve ────────────────────────────────────────────────
async function approveRole(req) {
  try {
    await teacherRequestsApi.approve(req.id)
    const found = roleRequests.value.find(r => r.id === req.id)
    if (found) found.status = 'approved'
  } catch (e) {
    alert(e.response?.data?.message || 'Не удалось одобрить заявку')
  }
}

async function approveChange(req) {
  try {
    await changeRequestsApi.approve(req.id)
    const found = changeRequests.value.find(r => r.id === req.id)
    if (found) found.status = 'approved'
  } catch (e) {
    alert(e.response?.data?.message || 'Не удалось одобрить заявку')
  }
}

// ── Change detail modal ────────────────────────────────────
const changeDetail = reactive({ open: false, req: null })

function openChangeDetail(req) {
  changeDetail.req = req
  changeDetail.open = true
}

function formatChanges(raw) {
  if (!raw) return '—'
  try {
    const obj = typeof raw === 'string' ? JSON.parse(raw) : raw
    return Object.entries(obj)
      .map(([k, v]) => `${k}: ${v}`)
      .join('\n')
  } catch {
    return String(raw)
  }
}
</script>

<template>
  <div class="requests-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-wrap">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">

        <!-- Top bar -->
        <div class="page-top">
          <h1 class="page-title">Центр заявок</h1>
          <div class="page-actions">
            <button class="btn-ghost" @click="resetFilters">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <path d="M3 6h18M7 12h10M11 18h2"/>
              </svg>
              Сбросить фильтры
            </button>
            <button class="btn-ghost" @click="refresh">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <path d="M23 4v6h-6M1 20v-6h6"/>
                <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/>
              </svg>
              Обновить
            </button>
          </div>
        </div>

        <!-- Tabs -->
        <div class="tabs-bar">
          <button class="tab-btn" :class="{ active: activeTab === 'retakes' }" @click="activeTab = 'retakes'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 12h-6l-2 3H10l-2-3H2"/><path d="M5.45 5.11L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.45-6.89A2 2 0 0016.76 4H7.24a2 2 0 00-1.79 1.11z"/>
            </svg>
            Заявки на пересдачи
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'changes' }" @click="activeTab = 'changes'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
            Изменения пересдач
            <span v-if="pendingChangesCount" class="tab-badge">{{ pendingChangesCount }}</span>
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'roles' }" @click="activeTab = 'roles'">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/>
            </svg>
            Смена роли
            <span v-if="pendingRolesCount" class="tab-badge">{{ pendingRolesCount }}</span>
          </button>
        </div>

        <!-- Global error -->
        <div v-if="loadError" class="load-error">{{ loadError }}</div>

        <!-- ── Tab: Retake requests (not implemented yet) ── -->
        <div v-if="activeTab === 'retakes'">
          <div class="empty-state empty-state--page">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
              <path d="M22 12h-6l-2 3H10l-2-3H2"/><path d="M5.45 5.11L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.45-6.89A2 2 0 0016.76 4H7.24a2 2 0 00-1.79 1.11z"/>
            </svg>
            <p>Раздел в разработке</p>
            <span>Функциональность заявок на пересдачи будет добавлена позже</span>
          </div>
        </div>

        <!-- ── Tab: Retake change requests ── -->
        <div v-else-if="activeTab === 'changes'" class="table-wrap">
          <div v-if="loading" class="empty-state">Загрузка…</div>
          <table v-else class="data-table">
            <thead>
              <tr class="head-row">
                <th>Преподаватель</th>
                <th>ID пересдачи</th>
                <th>Запрошенные изменения</th>
                <th>Причина</th>
                <th>Дата заявки</th>
                <th>Статус</th>
                <th>Действия</th>
              </tr>
              <tr class="filter-row">
                <th><input class="col-filter" v-model="changeF.user" placeholder="Поиск..." /></th>
                <th><input class="col-filter" v-model="changeF.retake" placeholder="Поиск..." /></th>
                <th></th>
                <th></th>
                <th></th>
                <th>
                  <select class="col-filter col-select" v-model="changeF.status">
                    <option value="all">Все</option>
                    <option value="pending">Ожидает</option>
                    <option value="approved">Одобрено</option>
                    <option value="rejected">Отклонено</option>
                  </select>
                </th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="req in filteredChanges" :key="req.id">
                <td>
                  <div class="user-cell">
                    <span class="user-name">{{ fullName(req.requester_first_name, req.requester_last_name, req.requester_middle_name) }}</span>
                    <span class="user-email">{{ req.requester_email }}</span>
                  </div>
                </td>
                <td class="td-mono">{{ req.retake_id.slice(0, 8) }}…</td>
                <td>
                  <button class="btn-outline btn-sm" @click="openChangeDetail(req)">Подробнее</button>
                </td>
                <td class="td-muted">{{ req.reason || '—' }}</td>
                <td class="td-date">{{ fmt(req.created_at) }}</td>
                <td>
                  <span class="status-badge"
                    :style="{ color: STATUS_MAP[req.status]?.color, background: STATUS_MAP[req.status]?.bg }">
                    {{ STATUS_MAP[req.status]?.label }}
                  </span>
                </td>
                <td>
                  <div v-if="req.status === 'pending'" class="row-actions">
                    <button class="btn-danger btn-sm" @click="openReject(req, 'change')">Отклонить</button>
                    <button class="btn-primary btn-sm" @click="approveChange(req)">Одобрить</button>
                  </div>
                </td>
              </tr>
              <tr v-if="filteredChanges.length === 0">
                <td colspan="7">
                  <div class="empty-state">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                      <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                      <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                    </svg>
                    <p>Заявок не найдено</p>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- ── Tab: Role requests ── -->
        <div v-else-if="activeTab === 'roles'" class="table-wrap">
          <div v-if="loading" class="empty-state">Загрузка…</div>
          <table v-else class="data-table">
            <thead>
              <tr class="head-row">
                <th>ФИО</th>
                <th>Email</th>
                <th>Мотивация</th>
                <th>Дата заявки</th>
                <th>Статус</th>
                <th>Действия</th>
              </tr>
              <tr class="filter-row">
                <th><input class="col-filter" v-model="roleF.user" placeholder="Поиск..." /></th>
                <th></th>
                <th></th>
                <th></th>
                <th>
                  <select class="col-filter col-select" v-model="roleF.status">
                    <option value="all">Все</option>
                    <option value="pending">Ожидает</option>
                    <option value="approved">Одобрено</option>
                    <option value="rejected">Отклонено</option>
                  </select>
                </th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="req in filteredRoles" :key="req.id">
                <td>
                  <span class="user-name">{{ fullName(req.user_first_name, req.user_last_name, req.user_middle_name) }}</span>
                </td>
                <td class="td-muted">{{ req.user_email }}</td>
                <td class="td-muted">{{ req.reason || '—' }}</td>
                <td class="td-date">{{ fmt(req.created_at) }}</td>
                <td>
                  <span class="status-badge"
                    :style="{ color: STATUS_MAP[req.status]?.color, background: STATUS_MAP[req.status]?.bg }">
                    {{ STATUS_MAP[req.status]?.label }}
                  </span>
                </td>
                <td>
                  <div v-if="req.status === 'pending'" class="row-actions">
                    <button class="btn-danger btn-sm" @click="openReject(req, 'role')">Отклонить</button>
                    <button class="btn-primary btn-sm" @click="approveRole(req)">Одобрить</button>
                  </div>
                  <span v-else-if="req.decision_reason" class="td-muted" style="font-size:12px">
                    {{ req.decision_reason }}
                  </span>
                </td>
              </tr>
              <tr v-if="filteredRoles.length === 0">
                <td colspan="6">
                  <div class="empty-state">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                      <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/>
                      <path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/>
                    </svg>
                    <p>Заявок не найдено</p>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

      </main>
    </div>

    <!-- ── Modal: Change detail ── -->
    <Transition name="modal">
      <div v-if="changeDetail.open" class="modal-overlay" @click.self="changeDetail.open = false">
        <div class="modal-card modal-card--sm">
          <div class="modal-head">
            <h3 class="modal-title">Запрошенные изменения</h3>
            <button class="modal-close" @click="changeDetail.open = false" aria-label="Закрыть">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                <path d="M18 6L6 18M6 6l12 12"/>
              </svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="detail-row">
              <span class="detail-label">Пересдача</span>
              <span class="detail-value td-mono">{{ changeDetail.req?.retake_id }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Преподаватель</span>
              <span class="detail-value">
                {{ fullName(changeDetail.req?.requester_first_name, changeDetail.req?.requester_last_name, changeDetail.req?.requester_middle_name) }}
              </span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Причина</span>
              <span class="detail-value">{{ changeDetail.req?.reason || '—' }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Изменения</span>
              <pre class="changes-pre">{{ formatChanges(changeDetail.req?.requested_changes) }}</pre>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn-ghost" @click="changeDetail.open = false">Закрыть</button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- ── Modal: Reject ── -->
    <Transition name="modal">
      <div v-if="rejectModal.open" class="modal-overlay" @click.self="rejectModal.open = false">
        <div class="modal-card modal-card--sm">
          <div class="modal-head">
            <h3 class="modal-title">Отклонить заявку</h3>
            <button class="modal-close" @click="rejectModal.open = false" aria-label="Закрыть">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                <path d="M18 6L6 18M6 6l12 12"/>
              </svg>
            </button>
          </div>
          <div class="modal-body">
            <p class="reject-hint">Укажите причину отклонения (необязательно)</p>
            <textarea class="input input--area" v-model="rejectModal.reason" placeholder="Причина отклонения..." />
            <p v-if="rejectModal.error" class="form-error">{{ rejectModal.error }}</p>
          </div>
          <div class="modal-footer">
            <button class="btn-ghost" @click="rejectModal.open = false">Отмена</button>
            <button class="btn-danger-solid" @click="confirmReject" :disabled="rejectModal.submitting">
              {{ rejectModal.submitting ? 'Отклоняем…' : 'Отклонить' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

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

.page-wrap {
  min-height: 100dvh;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

.main {
  flex: 1;
  padding: 24px;
  display: flex;
  flex-direction: column;
}

/* ── Page top ── */
.page-top {
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; margin-bottom: 24px; flex-wrap: wrap;
}
.page-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 22px; font-weight: 700; color: #3C38B6; margin: 0;
}
.page-actions { display: flex; gap: 8px; }

.btn-ghost {
  display: flex; align-items: center; gap: 6px;
  height: 36px; padding: 0 14px;
  background: var(--card); border: 1.5px solid var(--line);
  border-radius: var(--radius);
  font: 13px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; white-space: nowrap;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-ghost:hover { border-color: var(--brand); color: var(--brand); background: rgba(59,63,224,.04); }
.btn-ghost svg { width: 15px; height: 15px; flex-shrink: 0; }

/* ── Load error ── */
.load-error {
  background: rgba(220,38,38,.08); border: 1px solid rgba(220,38,38,.2);
  border-radius: var(--radius); padding: 12px 16px;
  color: #dc2626; font-size: 13px; margin-bottom: 16px;
}

/* ── Tabs ── */
.tabs-bar {
  display: flex; gap: 6px;
  background: var(--card); border-radius: 14px; padding: 6px;
  box-shadow: var(--shadow); margin-bottom: 20px;
}
.tab-btn {
  flex: 1; height: 44px; border: none; border-radius: 10px;
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink-soft); background: transparent;
  cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px;
  white-space: nowrap; flex-shrink: 0;
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
  min-width: 20px; height: 20px; padding: 0 5px;
  background: rgba(245,158,11,.25); color: #b45309;
  border-radius: 10px; font: 700 11px/1 'Inter', sans-serif;
}
.tab-btn.active .tab-badge { background: rgba(255,255,255,.25); color: #fff; }

/* ── Table ── */
.table-wrap {
  background: var(--card);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  border: 1px solid var(--line);
  overflow: auto;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font: 13px/1.4 'Inter', sans-serif;
}

.head-row th {
  background: #f8f9fb;
  color: var(--ink-soft);
  font-weight: 600;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: .05em;
  padding: 13px 14px;
  text-align: left;
  border-bottom: 1px solid var(--line);
  white-space: nowrap;
  position: sticky;
  top: 0;
  z-index: 2;
}

.filter-row th {
  background: #f3f4f7;
  padding: 7px 10px;
  border-bottom: 2px solid var(--line);
  position: sticky;
  top: 40px;
  z-index: 1;
}

.col-filter {
  width: 100%;
  appearance: none;
  height: 28px;
  border: 1.5px solid var(--line);
  border-radius: 6px;
  background: #fff;
  padding: 0 8px;
  font: 12px/1 'Inter', sans-serif;
  color: var(--ink);
  outline: none;
  transition: border-color .15s, box-shadow .15s;
  min-width: 70px;
}
.col-filter:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.col-filter::placeholder { color: #b7b9c2; }
.col-select { cursor: pointer; }

.data-table tbody tr {
  border-bottom: 1px solid var(--line);
  transition: background .1s;
}
.data-table tbody tr:last-child { border-bottom: none; }
.data-table tbody tr:hover { background: rgba(59,63,224,.025); }

.data-table tbody td {
  padding: 13px 14px;
  vertical-align: middle;
  color: var(--ink);
}

.td-date  { white-space: nowrap; font-size: 12px; color: var(--ink-soft); }
.td-muted { font-size: 12px; color: var(--ink-soft); max-width: 180px; }
.td-mono  { font: 12px/1 monospace; color: var(--ink-soft); }

.user-cell { display: flex; flex-direction: column; gap: 2px; }
.user-name  { font: 500 13px/1.3 'Inter', sans-serif; color: var(--ink); }
.user-email { font: 12px/1.3 'Inter', sans-serif; color: var(--ink-soft); }

.status-badge {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 4px 10px; border-radius: 20px;
  font: 600 12px/1 'Inter', sans-serif; white-space: nowrap;
}
.status-badge::before {
  content: ''; width: 6px; height: 6px;
  border-radius: 50%; background: currentColor; opacity: .7; flex-shrink: 0;
}

.row-actions { display: flex; align-items: center; gap: 6px; white-space: nowrap; }

.btn-sm { height: 30px !important; padding: 0 12px !important; font-size: 12px !important; }

.btn-primary {
  height: 36px; padding: 0 20px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 4px 14px -6px rgba(91,59,217,.6);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-primary:hover { transform: scale(1.03); box-shadow: 0 4px 10px -4px rgba(91,59,217,.75); }

.btn-outline {
  height: 36px; padding: 0 16px;
  background: none; border: 1.5px solid var(--line); border-radius: var(--radius);
  font: 500 13px/1 'Inter', sans-serif; color: var(--ink-soft); cursor: pointer;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-outline:hover { border-color: var(--brand); color: var(--brand); background: rgba(59,63,224,.04); }

.btn-danger {
  height: 36px; padding: 0 16px;
  background: none; border: 1.5px solid rgba(220,38,38,.3); border-radius: var(--radius);
  font: 500 13px/1 'Inter', sans-serif; color: #dc2626; cursor: pointer;
  transition: border-color .15s, background .15s;
}
.btn-danger:hover { border-color: #dc2626; background: rgba(220,38,38,.06); }

.btn-danger-solid {
  height: 36px; padding: 0 20px; border: none; border-radius: var(--radius);
  background: #dc2626; color: #fff;
  font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 4px 14px -6px rgba(220,38,38,.5);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-danger-solid:hover:not(:disabled) { transform: scale(1.02); box-shadow: 0 4px 10px -4px rgba(220,38,38,.65); }
.btn-danger-solid:disabled { opacity: .6; cursor: not-allowed; }

/* ── Empty state ── */
.empty-state {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 8px; padding: 48px 24px; color: var(--ink-soft); text-align: center;
}
.empty-state svg { width: 44px; height: 44px; opacity: .3; }
.empty-state p { font: 600 15px/1 'Inter', sans-serif; margin: 0; }
.empty-state--page {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); min-height: 300px;
}
.empty-state--page span { font-size: 13px; max-width: 300px; line-height: 1.5; }

/* ── Modal ── */
.modal-overlay {
  position: fixed; inset: 0;
  background: rgba(10,12,30,.5); z-index: 200;
  backdrop-filter: blur(3px); -webkit-backdrop-filter: blur(3px);
  display: grid; place-items: center; padding: 20px;
}
.modal-card {
  background: var(--card); border-radius: 16px;
  box-shadow: 0 24px 56px -12px rgba(10,12,30,.3);
  width: min(560px, 100%); max-height: 88vh;
  display: flex; flex-direction: column; overflow: hidden;
}
.modal-card--sm { max-width: 480px; }
.modal-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 24px; border-bottom: 1px solid var(--line); flex-shrink: 0;
}
.modal-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0;
}
.modal-close {
  width: 32px; height: 32px; border-radius: 8px;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; flex-shrink: 0;
  color: var(--ink-soft); transition: background .15s, color .15s;
}
.modal-close:hover { background: var(--bg); color: var(--ink); }
.modal-close svg { width: 16px; height: 16px; }

.modal-body {
  flex: 1; overflow-y: auto; padding: 20px 24px;
  display: flex; flex-direction: column; gap: 12px;
}
.modal-body::-webkit-scrollbar { width: 4px; }
.modal-body::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }

.detail-row { display: flex; gap: 12px; align-items: flex-start; }
.detail-label {
  font: 500 13px/1.5 'Inter', sans-serif; color: var(--ink-soft);
  min-width: 120px; flex-shrink: 0;
}
.detail-value { font: 13px/1.5 'Inter', sans-serif; color: var(--ink); word-break: break-all; }

.changes-pre {
  flex: 1; font: 12px/1.6 monospace; color: var(--ink);
  background: #f8f9fb; border: 1px solid var(--line); border-radius: 6px;
  padding: 10px 12px; margin: 0; white-space: pre-wrap; word-break: break-all;
}

.modal-footer {
  display: flex; gap: 8px; justify-content: flex-end;
  padding: 14px 24px; border-top: 1px solid var(--line); flex-shrink: 0;
}

.input {
  appearance: none; height: 38px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.input::placeholder { color: #b7b9c2; }
.input--area { height: auto; min-height: 80px; padding: 8px 12px; line-height: 1.5; resize: vertical; }

.reject-hint { font: 13px/1.5 'Inter', sans-serif; color: var(--ink-soft); margin: 0 0 10px; }
.form-error { color: #d63a51; font-size: 13px; margin: 4px 0 0; }

/* ── Transition ── */
.modal-enter-active { transition: opacity .2s var(--ease); }
.modal-leave-active { transition: opacity .15s var(--ease); }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .modal-card { transition: transform .25s var(--ease); }
.modal-leave-active  .modal-card { transition: transform .2s  var(--ease); }
.modal-enter-from .modal-card, .modal-leave-to .modal-card { transform: scale(.96) translateY(12px); }

/* ── Responsive ── */
@media (max-width: 768px) {
  .main { padding: 16px; }
  .page-top { flex-direction: column; align-items: flex-start; }
  .detail-row { flex-direction: column; gap: 4px; }
  .detail-label { min-width: unset; }
}
</style>
