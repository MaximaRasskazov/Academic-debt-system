<script setup>
import { ref, reactive, computed } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'

const sidebarOpen = ref(false)
const activeTab = ref('retakes')

// ── Mock data ──────────────────────────────────────────────
const retakeRequests = ref([
  {
    id: 1, number: '001',
    teacher: 'Смирнов Андрей Валерьевич',
    subject: 'Математический анализ',
    student: 'Иванов Иван Иванович', group: 'ИС-31',
    submittedAt: '2026-05-20T14:32', preferredDate: '28.05.2026',
    status: 'pending', attempts: 1,
    reason: 'Болезнь в период сессии. Медицинская справка прилагается.',
  },
  {
    id: 2, number: '002',
    teacher: 'Гришина Ольга Михайловна',
    subject: 'Физика',
    student: 'Петрова Анна Сергеевна', group: 'ИС-32',
    submittedAt: '2026-05-21T09:15', preferredDate: '30.05.2026',
    status: 'assigned', attempts: 2,
    reason: 'Пропуск по уважительной причине (семейные обстоятельства).',
    assignedDate: '30.05.2026', assignedTime: '14:00', assignedBuilding: '2', assignedRoom: '308',
  },
  {
    id: 3, number: '003',
    teacher: 'Смирнов Андрей Валерьевич',
    subject: 'Линейная алгебра',
    student: 'Сидоров Алексей Петрович', group: 'ИТ-21',
    submittedAt: '2026-05-21T11:47', preferredDate: '05.06.2026',
    status: 'rejected', attempts: 1,
    reason: 'Не явился на экзамен без предупреждения.',
    rejectReason: 'Нет оснований для пересдачи.',
  },
  {
    id: 4, number: '004',
    teacher: 'Карпов Игорь Степанович',
    subject: 'Теория вероятностей',
    student: 'Козлова Мария Дмитриевна', group: 'ИС-31',
    submittedAt: '2026-05-22T08:55', preferredDate: '02.06.2026',
    status: 'pending', attempts: 1,
    reason: 'Технические проблемы при подключении во время онлайн-экзамена.',
  },
])

const roleRequests = ref([
  {
    id: 1, number: '101', user: 'Козлов Дмитрий Николаевич', email: 'd.kozlov@university.ru',
    currentRole: 'STUDENT', requestedRole: 'TEACHER',
    submittedAt: '2026-05-19T10:00', status: 'pending',
    comment: 'Являюсь аспирантом кафедры математики, веду практические занятия.',
  },
  {
    id: 2, number: '102', user: 'Жукова Екатерина Олеговна', email: 'e.zhukova@university.ru',
    currentRole: 'TEACHER', requestedRole: 'DEAN',
    submittedAt: '2026-05-20T16:30', status: 'approved',
    comment: '',
  },
])

// ── Helpers ────────────────────────────────────────────────
const STATUS_MAP = {
  pending:  { label: 'Ожидает',   color: '#d97706', bg: 'rgba(245,158,11,.12)'  },
  assigned: { label: 'Назначено', color: '#059669', bg: 'rgba(16,185,129,.12)'  },
  rejected: { label: 'Отклонено', color: '#dc2626', bg: 'rgba(239,68,68,.12)'   },
  approved: { label: 'Одобрено',  color: '#059669', bg: 'rgba(16,185,129,.12)'  },
}
const ROLE_MAP = { STUDENT: 'Студент', TEACHER: 'Преподаватель', DEAN: 'Деканат' }

function fmt(iso) {
  const d = new Date(iso)
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: '2-digit' }) + ' ' +
    d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
}

// ── Column filters: retakes ────────────────────────────────
const rf = reactive({ number: '', teacher: '', subject: '', student: '', group: '', status: 'all' })

// ── Column filters: roles ──────────────────────────────────
const roleF = reactive({ user: '', email: '', currentRole: 'all', requestedRole: 'all', status: 'all' })

function resetFilters() {
  rf.number = ''; rf.teacher = ''; rf.subject = ''
  rf.student = ''; rf.group = ''; rf.status = 'all'
  roleF.user = ''; roleF.email = ''
  roleF.currentRole = 'all'; roleF.requestedRole = 'all'; roleF.status = 'all'
}

function refresh() { /* TODO: API */ }

const filteredRetakes = computed(() =>
  retakeRequests.value.filter(r => {
    if (rf.number  && !r.number.includes(rf.number)) return false
    if (rf.teacher && !r.teacher.toLowerCase().includes(rf.teacher.toLowerCase())) return false
    if (rf.subject && !r.subject.toLowerCase().includes(rf.subject.toLowerCase())) return false
    if (rf.student && !r.student.toLowerCase().includes(rf.student.toLowerCase())) return false
    if (rf.group   && !r.group.toLowerCase().includes(rf.group.toLowerCase())) return false
    if (rf.status !== 'all' && r.status !== rf.status) return false
    return true
  })
)

const filteredRoles = computed(() =>
  roleRequests.value.filter(r => {
    if (roleF.user  && !r.user.toLowerCase().includes(roleF.user.toLowerCase())) return false
    if (roleF.email && !r.email.toLowerCase().includes(roleF.email.toLowerCase())) return false
    if (roleF.currentRole  !== 'all' && r.currentRole  !== roleF.currentRole)  return false
    if (roleF.requestedRole !== 'all' && r.requestedRole !== roleF.requestedRole) return false
    if (roleF.status !== 'all' && r.status !== roleF.status) return false
    return true
  })
)

const pendingRetakesCount = computed(() => retakeRequests.value.filter(r => r.status === 'pending').length)
const pendingRolesCount   = computed(() => roleRequests.value.filter(r => r.status === 'pending').length)

// ── Detail modal ───────────────────────────────────────────
const detail = reactive({ open: false, req: null, editing: false, draft: null })

function openDetail(req) {
  detail.req = req
  detail.draft = { subject: req.subject, reason: req.reason, preferredDate: req.preferredDate }
  detail.editing = false
  detail.open = true
}
function saveEdit() {
  const found = retakeRequests.value.find(r => r.id === detail.req.id)
  if (found) Object.assign(found, detail.draft)
  Object.assign(detail.req, detail.draft)
  detail.editing = false
}

// ── Assign modal ───────────────────────────────────────────
const assign = reactive({ open: false, req: null, date: '', time: '', building: '', room: '', teacher: '' })

function openAssign(req) {
  assign.req = req
  assign.date = req.preferredDate || ''
  assign.time = ''
  assign.building = ''
  assign.room = ''
  assign.teacher = req.teacher || ''
  assign.open = true
}
function confirmAssign() {
  const found = retakeRequests.value.find(r => r.id === assign.req.id)
  if (found) {
    Object.assign(found, {
      status: 'assigned',
      assignedDate: assign.date,
      assignedTime: assign.time,
      assignedBuilding: assign.building,
      assignedRoom: assign.room,
    })
  }
  assign.open = false
}

// ── Reject modal ───────────────────────────────────────────
const rejectModal = reactive({ open: false, req: null, reason: '', type: 'retake' })

function openReject(req, type = 'retake') {
  rejectModal.req = req
  rejectModal.reason = ''
  rejectModal.type = type
  rejectModal.open = true
}
function confirmReject() {
  if (rejectModal.type === 'retake') {
    const found = retakeRequests.value.find(r => r.id === rejectModal.req.id)
    if (found) Object.assign(found, { status: 'rejected', rejectReason: rejectModal.reason })
  } else {
    const found = roleRequests.value.find(r => r.id === rejectModal.req.id)
    if (found) found.status = 'rejected'
  }
  rejectModal.open = false
}

function approveRole(req) {
  const found = roleRequests.value.find(r => r.id === req.id)
  if (found) found.status = 'approved'
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
            Заявки на пересдачи
            <span v-if="pendingRetakesCount" class="tab-badge">{{ pendingRetakesCount }}</span>
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'changes' }" @click="activeTab = 'changes'">
            Изменения пересдач
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'roles' }" @click="activeTab = 'roles'">
            Смена роли
            <span v-if="pendingRolesCount" class="tab-badge">{{ pendingRolesCount }}</span>
          </button>
        </div>

        <!-- ── Tab: Retake requests ── -->
        <div v-if="activeTab === 'retakes'" class="table-wrap">
          <table class="data-table">
            <thead>
              <tr class="head-row">
                <th>№</th>
                <th>Преподаватель</th>
                <th>Предмет</th>
                <th>Студент</th>
                <th>Группа</th>
                <th>Дата заявки</th>
                <th>Желаемая дата</th>
                <th>Попытка</th>
                <th>Статус</th>
                <th>Действия</th>
              </tr>
              <tr class="filter-row">
                <th><input class="col-filter" v-model="rf.number"  placeholder="Поиск..." /></th>
                <th><input class="col-filter" v-model="rf.teacher" placeholder="Поиск..." /></th>
                <th><input class="col-filter" v-model="rf.subject" placeholder="Поиск..." /></th>
                <th><input class="col-filter" v-model="rf.student" placeholder="Поиск..." /></th>
                <th><input class="col-filter" v-model="rf.group"   placeholder="Поиск..." /></th>
                <th></th>
                <th></th>
                <th></th>
                <th>
                  <select class="col-filter col-select" v-model="rf.status">
                    <option value="all">Все</option>
                    <option value="pending">Ожидает</option>
                    <option value="assigned">Назначено</option>
                    <option value="rejected">Отклонено</option>
                  </select>
                </th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="req in filteredRetakes" :key="req.id">
                <td class="td-num">#{{ req.number }}</td>
                <td>{{ req.teacher }}</td>
                <td>{{ req.subject }}</td>
                <td>{{ req.student }}</td>
                <td><span class="group-chip">{{ req.group }}</span></td>
                <td class="td-date">{{ fmt(req.submittedAt) }}</td>
                <td class="td-date">{{ req.preferredDate }}</td>
                <td class="td-center">№{{ req.attempts }}</td>
                <td>
                  <span class="status-badge"
                    :style="{ color: STATUS_MAP[req.status].color, background: STATUS_MAP[req.status].bg }">
                    {{ STATUS_MAP[req.status].label }}
                  </span>
                </td>
                <td>
                  <div class="row-actions">
                    <button class="btn-outline btn-sm" @click="openDetail(req)">Подробнее</button>
                    <template v-if="req.status === 'pending'">
                      <button class="btn-danger btn-sm" @click="openReject(req, 'retake')">Отклонить</button>
                      <button class="btn-primary btn-sm" @click="openAssign(req)">Назначить</button>
                    </template>
                  </div>
                </td>
              </tr>
              <tr v-if="filteredRetakes.length === 0">
                <td colspan="10">
                  <div class="empty-state">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                      <path d="M9 12h6M9 16h6M7 4h10a2 2 0 012 2v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z"/>
                    </svg>
                    <p>Заявок не найдено</p>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- ── Tab: Changes (empty) ── -->
        <div v-else-if="activeTab === 'changes'">
          <div class="empty-state empty-state--page">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
              <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
              <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
            <p>Раздел в разработке</p>
            <span>Функциональность изменения пересдач будет добавлена позже</span>
          </div>
        </div>

        <!-- ── Tab: Role requests ── -->
        <div v-else-if="activeTab === 'roles'" class="table-wrap">
          <table class="data-table">
            <thead>
              <tr class="head-row">
                <th>№</th>
                <th>ФИО</th>
                <th>Email</th>
                <th>Текущая роль</th>
                <th>Запрашиваемая роль</th>
                <th>Дата заявки</th>
                <th>Статус</th>
                <th>Действия</th>
              </tr>
              <tr class="filter-row">
                <th></th>
                <th><input class="col-filter" v-model="roleF.user"  placeholder="Поиск..." /></th>
                <th><input class="col-filter" v-model="roleF.email" placeholder="Поиск..." /></th>
                <th>
                  <select class="col-filter col-select" v-model="roleF.currentRole">
                    <option value="all">Все</option>
                    <option value="STUDENT">Студент</option>
                    <option value="TEACHER">Преподаватель</option>
                    <option value="DEAN">Деканат</option>
                  </select>
                </th>
                <th>
                  <select class="col-filter col-select" v-model="roleF.requestedRole">
                    <option value="all">Все</option>
                    <option value="STUDENT">Студент</option>
                    <option value="TEACHER">Преподаватель</option>
                    <option value="DEAN">Деканат</option>
                  </select>
                </th>
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
                <td class="td-num">#{{ req.number }}</td>
                <td>{{ req.user }}</td>
                <td class="td-muted">{{ req.email }}</td>
                <td><span class="role-chip">{{ ROLE_MAP[req.currentRole] }}</span></td>
                <td><span class="role-chip role-chip--new">{{ ROLE_MAP[req.requestedRole] }}</span></td>
                <td class="td-date">{{ fmt(req.submittedAt) }}</td>
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
                </td>
              </tr>
              <tr v-if="filteredRoles.length === 0">
                <td colspan="8">
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

    <!-- ── Modal: Detail ── -->
    <Transition name="modal">
      <div v-if="detail.open" class="modal-overlay" @click.self="detail.open = false">
        <div class="modal-card">
          <div class="modal-head">
            <h3 class="modal-title">Заявка #{{ detail.req?.number }}</h3>
            <div class="modal-head-right">
              <button
                v-if="!detail.editing && detail.req?.status === 'pending'"
                class="btn-edit" @click="detail.editing = true"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                  <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
                Редактировать
              </button>
              <button class="modal-close" @click="detail.open = false" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
            </div>
          </div>
          <div class="modal-body">
            <div class="detail-row">
              <span class="detail-label">Преподаватель</span>
              <span class="detail-value">{{ detail.req?.teacher }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Студент</span>
              <span class="detail-value">{{ detail.req?.student }}, {{ detail.req?.group }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Дисциплина</span>
              <span v-if="!detail.editing" class="detail-value">{{ detail.req?.subject }}</span>
              <input v-else class="detail-input" v-model="detail.draft.subject" />
            </div>
            <div class="detail-row">
              <span class="detail-label">Попытка</span>
              <span class="detail-value">№{{ detail.req?.attempts }}</span>
            </div>
            <div class="detail-row">
              <span class="detail-label">Желаемая дата</span>
              <span v-if="!detail.editing" class="detail-value">{{ detail.req?.preferredDate }}</span>
              <input v-else class="detail-input" v-model="detail.draft.preferredDate" placeholder="дд.мм.гггг" />
            </div>
            <div class="detail-row">
              <span class="detail-label">Причина</span>
              <span v-if="!detail.editing" class="detail-value">{{ detail.req?.reason }}</span>
              <textarea v-else class="detail-input detail-textarea" v-model="detail.draft.reason" />
            </div>
            <div v-if="detail.req?.status === 'assigned'" class="detail-row">
              <span class="detail-label">Пересдача назначена</span>
              <span class="detail-value">
                {{ detail.req?.assignedDate }}, {{ detail.req?.assignedTime }}
                · корп. {{ detail.req?.assignedBuilding }}, ауд. {{ detail.req?.assignedRoom }}
              </span>
            </div>
            <div v-if="detail.req?.status === 'rejected' && detail.req?.rejectReason" class="detail-row">
              <span class="detail-label">Причина отказа</span>
              <span class="detail-value detail-value--danger">{{ detail.req?.rejectReason }}</span>
            </div>
          </div>
          <div v-if="detail.editing" class="modal-footer">
            <button class="btn-ghost" @click="detail.editing = false">Отмена</button>
            <button class="btn-primary" @click="saveEdit">Сохранить</button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- ── Modal: Assign ── -->
    <Transition name="modal">
      <div v-if="assign.open" class="modal-overlay" @click.self="assign.open = false">
        <div class="modal-card">
          <div class="modal-head">
            <h3 class="modal-title">Назначить пересдачу</h3>
            <button class="modal-close" @click="assign.open = false" aria-label="Закрыть">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                <path d="M18 6L6 18M6 6l12 12"/>
              </svg>
            </button>
          </div>
          <div class="modal-body">
            <p class="assign-info">{{ assign.req?.student }} · {{ assign.req?.subject }}</p>
            <div class="assign-form">
              <div class="assign-field">
                <label>Дата</label>
                <input class="input" v-model="assign.date" placeholder="дд.мм.гггг" />
              </div>
              <div class="assign-field">
                <label>Время</label>
                <input class="input" v-model="assign.time" placeholder="09:00" />
              </div>
              <div class="assign-field">
                <label>Корпус</label>
                <input class="input" v-model="assign.building" placeholder="№ корпуса" />
              </div>
              <div class="assign-field">
                <label>Аудитория</label>
                <input class="input" v-model="assign.room" placeholder="№ аудитории" />
              </div>
              <div class="assign-field assign-field--full">
                <label>Преподаватель</label>
                <input class="input" v-model="assign.teacher" placeholder="ФИО преподавателя" />
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn-ghost" @click="assign.open = false">Отмена</button>
            <button class="btn-primary" @click="confirmAssign">Назначить</button>
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
          </div>
          <div class="modal-footer">
            <button class="btn-ghost" @click="rejectModal.open = false">Отмена</button>
            <button class="btn-danger-solid" @click="confirmReject">Отклонить</button>
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

/* ── Tabs ── */
.tabs-bar {
  display: flex; border-bottom: 2px solid var(--line);
  margin-bottom: 20px; overflow-x: auto;
}
.tab-btn {
  display: flex; align-items: center; gap: 8px; flex-shrink: 0;
  padding: 10px 20px; background: none; border: none;
  font: 14px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; white-space: nowrap;
  border-bottom: 2px solid transparent; margin-bottom: -2px;
  transition: color .15s;
}
.tab-btn:hover { color: var(--ink); }
.tab-btn.active { color: #3C38B6; font-weight: 600; border-bottom-color: #3C38B6; }
.tab-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 20px; padding: 0 6px;
  background: rgba(245,158,11,.15); color: #d97706;
  border-radius: 20px; font-size: 11px; font-weight: 700;
}

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

/* Label header row */
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

/* Filter header row */
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

/* Body */
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

.td-num   { font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft); white-space: nowrap; }
.td-date  { white-space: nowrap; font-size: 12px; color: var(--ink-soft); }
.td-center { text-align: center; font-size: 12px; color: var(--ink-soft); }
.td-muted { font-size: 12px; color: var(--ink-soft); }

.group-chip {
  display: inline-block; padding: 2px 8px; border-radius: 4px;
  background: rgba(59,63,224,.08); color: var(--brand-ink);
  font: 500 11px/1.4 'Inter', sans-serif; white-space: nowrap;
}

.status-badge {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 4px 10px; border-radius: 20px;
  font: 600 12px/1 'Inter', sans-serif; white-space: nowrap;
}
.status-badge::before {
  content: ''; width: 6px; height: 6px;
  border-radius: 50%; background: currentColor; opacity: .7; flex-shrink: 0;
}

.role-chip {
  display: inline-block; padding: 3px 10px; border-radius: 20px;
  background: rgba(107,114,128,.1); color: var(--ink-soft);
  font: 500 12px/1.4 'Inter', sans-serif; white-space: nowrap;
}
.role-chip--new { background: rgba(59,63,224,.1); color: var(--brand-ink); }

/* Row actions */
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
.btn-danger-solid:hover { transform: scale(1.02); box-shadow: 0 4px 10px -4px rgba(220,38,38,.65); }

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
  background: rgba(10,12,30,.45); z-index: 200;
  display: grid; place-items: center; padding: 24px;
}
.modal-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: 0 24px 48px -12px rgba(20,22,60,.25);
  width: min(540px, 100%); max-height: 82vh; overflow-y: auto; padding: 24px;
}
.modal-card--sm { width: min(400px, 100%); }
.modal-head {
  display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px;
}
.modal-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0;
}
.modal-head-right { display: flex; align-items: center; gap: 8px; }
.modal-close {
  background: none; border: none; cursor: pointer; padding: 4px;
  color: var(--ink-soft); border-radius: 6px; display: grid; place-items: center;
  transition: background .15s, color .15s;
}
.modal-close:hover { background: var(--bg); color: var(--ink); }
.modal-close svg { width: 18px; height: 18px; }

.btn-edit {
  display: flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 12px;
  background: none; border: 1.5px solid var(--line); border-radius: 6px;
  font: 500 12px/1 'Inter', sans-serif; color: var(--ink-soft); cursor: pointer;
  transition: border-color .15s, color .15s;
}
.btn-edit:hover { border-color: var(--brand); color: var(--brand); }
.btn-edit svg { width: 13px; height: 13px; }

.modal-body { display: flex; flex-direction: column; gap: 12px; }
.detail-row { display: flex; gap: 12px; align-items: flex-start; }
.detail-label {
  font: 500 13px/1.5 'Inter', sans-serif; color: var(--ink-soft);
  min-width: 140px; flex-shrink: 0;
}
.detail-value { font: 13px/1.5 'Inter', sans-serif; color: var(--ink); }
.detail-value--danger { color: #dc2626; }
.detail-input {
  flex: 1; appearance: none; border: 1.5px solid var(--line); border-radius: 6px;
  background: #fff; padding: 6px 10px;
  font: 13px/1.4 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.detail-input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.detail-textarea { resize: vertical; min-height: 80px; padding: 8px 10px; line-height: 1.5; }

.assign-info { font: 600 14px/1.4 'Inter', sans-serif; color: var(--ink); margin: 0 0 16px; }
.assign-form { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.assign-field { display: flex; flex-direction: column; gap: 6px; }
.assign-field--full { grid-column: 1 / -1; }
.assign-field label { font: 500 13px/1 'Inter', sans-serif; color: var(--ink); }

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

.modal-footer {
  display: flex; gap: 8px; justify-content: flex-end;
  margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--line);
}

/* ── Transition ── */
.modal-enter-active { transition: opacity .2s cubic-bezier(.2,.7,.2,1); }
.modal-leave-active { transition: opacity .15s cubic-bezier(.2,.7,.2,1); }
.modal-enter-from, .modal-leave-to { opacity: 0; }

/* ── Responsive ── */
@media (max-width: 768px) {
  .main { padding: 16px; }
  .page-top { flex-direction: column; align-items: flex-start; }
  .assign-form { grid-template-columns: 1fr; }
  .assign-field--full { grid-column: auto; }
  .detail-row { flex-direction: column; gap: 4px; }
  .detail-label { min-width: unset; }
}
</style>
