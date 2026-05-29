<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import http from '../api/client'

const sidebarOpen = ref(false)

/* ─── State ──────────────────────────────────────────────────── */
const users      = ref([])
const total      = ref(0)
const allGroups  = ref([])
const loading    = ref(false)
const forbidden  = ref(false)

const PAGE_SIZE   = 20
const searchQuery = ref('')
const filterRole  = ref('')   // '' = все, 'student', 'teacher', 'dean'
const filterGroup = ref('')
const currentPage = ref(1)

/* ─── Fetch ──────────────────────────────────────────────────── */
async function fetchUsers() {
  loading.value = true
  forbidden.value = false
  try {
    const params = {
      limit:  PAGE_SIZE,
      offset: (currentPage.value - 1) * PAGE_SIZE,
    }
    if (filterRole.value)          params.role       = filterRole.value
    if (filterGroup.value)         params.group_name = filterGroup.value
    if (searchQuery.value.trim())  params.search     = searchQuery.value.trim()

    const { data } = await http.get('/api/users', { params })
    users.value = (data.items ?? []).map(normalizeUser)
    total.value = data.total ?? 0
  } catch (e) {
    if (e.response?.status === 403) forbidden.value = true
    users.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function normalizeUser(u) {
  const role = (u.roles?.[0]?.slug ?? 'student').toUpperCase()
  return {
    id:         u.id,
    role,
    lastName:   u.last_name   ?? '',
    firstName:  u.first_name  ?? '',
    middleName: u.middle_name ?? '',
    group:      u.group_name  ?? null,
    email:      u.email,
    avatar:     null,
  }
}

/* ─── Watchers ───────────────────────────────────────────────── */
watch([filterRole, filterGroup], () => { currentPage.value = 1; fetchUsers() })
watch(currentPage, fetchUsers)

let searchTimer = null
watch(searchQuery, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { currentPage.value = 1; fetchUsers() }, 350)
})

/* ─── Init ───────────────────────────────────────────────────── */
onMounted(async () => {
  try {
    const { data } = await http.get('/api/users', { params: { role: 'student', limit: 200 } })
    allGroups.value = [
      ...new Set((data.items ?? []).filter(u => u.group_name).map(u => u.group_name)),
    ].sort()
  } catch {}
  fetchUsers()
})

/* ─── Pagination ─────────────────────────────────────────────── */
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const rangeStart = computed(() => total.value === 0 ? 0 : (currentPage.value - 1) * PAGE_SIZE + 1)
const rangeEnd   = computed(() => Math.min(currentPage.value * PAGE_SIZE, total.value))

const visiblePages = computed(() => {
  const t = totalPages.value, c = currentPage.value
  if (t <= 7) return Array.from({ length: t }, (_, i) => i + 1)
  const pages = [1]
  if (c > 3) pages.push('…')
  for (let i = Math.max(2, c - 1); i <= Math.min(t - 1, c + 1); i++) pages.push(i)
  if (c < t - 2) pages.push('…')
  pages.push(t)
  return pages
})

function goPage(p) {
  if (typeof p !== 'number') return
  currentPage.value = Math.max(1, Math.min(totalPages.value, p))
}

/* ─── Helpers ────────────────────────────────────────────────── */
function fullName(u) {
  return [u.lastName, u.firstName, u.middleName].filter(Boolean).join(' ')
}

function roleLabel(u) {
  if (u.role === 'DEAN')    return 'Деканат'
  if (u.role === 'TEACHER') return 'Преподаватель'
  return u.group || 'Студент'
}

const ROLE_STYLE = {
  DEAN:    { bg: 'rgba(217,119,6,.1)',  color: '#b45309' },
  TEACHER: { bg: 'rgba(139,61,240,.1)', color: '#7c22d6' },
  STUDENT: { bg: 'rgba(59,63,224,.1)',  color: '#3b3fe0' },
  ADMIN:   { bg: 'rgba(16,185,129,.1)', color: '#065f46' },
}
const DEFAULT_ROLE_STYLE = { bg: 'rgba(107,114,128,.1)', color: '#374151' }

function roleStyle(role) { return ROLE_STYLE[role] ?? DEFAULT_ROLE_STYLE }

const AVATAR_PALETTE = [
  '#3b3fe0','#e63c5a','#f59e0b','#10b981',
  '#8b3df0','#0ea5e9','#ec4899','#14b8a6',
  '#f97316','#6366f1','#84cc16','#06b6d4',
]

function avatarBg(u) {
  const key = u.lastName + u.firstName
  let h = 0
  for (const c of key) h = ((h << 5) - h + c.charCodeAt(0)) >>> 0
  return AVATAR_PALETTE[h % AVATAR_PALETTE.length]
}

function initials(u) {
  return ((u.lastName?.[0] ?? '') + (u.firstName?.[0] ?? '')).toUpperCase()
}
</script>

<template>
  <div class="users-root">
    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-users">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <!-- ── Заголовок ── -->
      <div class="page-bar">
        <div class="page-bar-left">
          <h1 class="page-title">Пользователи</h1>
          <span v-if="total > 0" class="total-chip">{{ total }} в системе</span>
        </div>
      </div>

      <!-- ── Фильтры ── -->
      <div class="filters-bar">

        <!-- Поиск -->
        <div class="search-wrap">
          <svg class="search-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
          </svg>
          <input
            class="search-input"
            type="text"
            v-model="searchQuery"
            placeholder="Поиск по ФИО, почте или группе..."
          />
          <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
          </button>
        </div>

        <!-- Роли -->
        <div class="chips">
          <button class="chip" :class="{ active: filterRole === '' }" @click="filterRole = ''">
            Все
          </button>
          <button class="chip" :class="{ active: filterRole === 'student' }" @click="filterRole = 'student'; filterGroup = ''">
            Студенты
          </button>
          <button class="chip" :class="{ active: filterRole === 'teacher' }" @click="filterRole = 'teacher'; filterGroup = ''">
            Преподаватели
          </button>
          <button class="chip" :class="{ active: filterRole === 'dean' }" @click="filterRole = 'dean'; filterGroup = ''">
            Деканат
          </button>
        </div>

        <!-- Группа (только когда фильтр = студенты или все) -->
        <div v-if="filterRole !== 'teacher' && filterRole !== 'dean'" class="group-select-wrap">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/>
            <path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/>
          </svg>
          <select class="group-select" v-model="filterGroup">
            <option value="">Все группы</option>
            <option v-for="g in allGroups" :key="g" :value="g">{{ g }}</option>
          </select>
        </div>

      </div>

      <!-- ── Счётчик результатов ── -->
      <div class="results-bar">
        <span v-if="loading" class="results-text results-empty">Загрузка…</span>
        <span v-else-if="total > 0" class="results-text">
          Показано&nbsp;<strong>{{ rangeStart }}–{{ rangeEnd }}</strong>&nbsp;из&nbsp;<strong>{{ total }}</strong>
        </span>
        <span v-else-if="!forbidden" class="results-text results-empty">Никого не найдено</span>
      </div>

      <!-- ── 403 ── -->
      <div v-if="forbidden" class="empty-state">
        <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="#d7d9e0" stroke-width="1.2" stroke-linecap="round">
          <circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/>
        </svg>
        <p class="empty-title">Нет доступа</p>
        <p class="empty-sub">Просмотр списка пользователей доступен только сотрудникам деканата</p>
      </div>

      <!-- ── Сетка карточек ── -->
      <div class="users-grid" v-else-if="!loading && users.length">
        <div
          v-for="u in users"
          :key="u.id"
          class="user-card"
        >
          <!-- Аватар -->
          <div class="u-avatar" :style="{ background: avatarBg(u) }">
            <img v-if="u.avatar" :src="u.avatar" :alt="fullName(u)" />
            <span v-else class="u-initials">{{ initials(u) }}</span>
          </div>

          <!-- ФИО -->
          <div class="u-name">{{ u.lastName }} {{ u.firstName }}<br>{{ u.middleName }}</div>

          <!-- Роль / группа -->
          <span
            class="u-badge"
            :style="{ background: roleStyle(u.role).bg, color: roleStyle(u.role).color }"
          >{{ roleLabel(u) }}</span>

          <!-- Почта -->
          <div class="u-email">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/>
            </svg>
            <span>{{ u.email }}</span>
          </div>
        </div>
      </div>

      <!-- Пустое состояние -->
      <div v-else-if="!loading && !forbidden" class="empty-state">
        <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="#d7d9e0" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
        </svg>
        <p class="empty-title">Никого не найдено</p>
        <p class="empty-sub">Попробуйте изменить запрос или сбросить фильтры</p>
        <button class="btn-reset" @click="searchQuery = ''; filterRole = ''; filterGroup = ''">Сбросить фильтры</button>
      </div>

      <!-- ── Пагинация ── -->
      <div v-if="totalPages > 1" class="pagination">
        <button
          class="pg-btn"
          :disabled="currentPage === 1"
          @click="goPage(currentPage - 1)"
          title="Предыдущая страница"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M15 18l-6-6 6-6"/></svg>
        </button>

        <template v-for="p in visiblePages" :key="p + '-' + currentPage">
          <span v-if="p === '…'" class="pg-dots">…</span>
          <button
            v-else
            class="pg-btn"
            :class="{ active: p === currentPage }"
            @click="goPage(p)"
          >{{ p }}</button>
        </template>

        <button
          class="pg-btn"
          :disabled="currentPage === totalPages"
          @click="goPage(currentPage + 1)"
          title="Следующая страница"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M9 18l6-6-6-6"/></svg>
        </button>
      </div>

      <div class="page-bottom-space" />

    </div>
  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

/* ── Variables ───────────────────────────────────────────────── */
.users-root {
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

.page-users {
  min-height: 100dvh;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

/* ── Page bar ────────────────────────────────────────────────── */
.page-bar {
  display: flex; align-items: center; gap: 12px;
  padding: 20px 24px 0;
}
.page-bar-left { display: flex; align-items: center; gap: 12px; }
.page-title {
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-size: 20px; font-weight: 700; color: #3C38B6; margin: 0;
}
.total-chip {
  padding: 4px 12px; border-radius: 20px;
  background: rgba(59,63,224,.1); color: var(--brand);
  font: 600 12px/1 'Inter', sans-serif;
}

/* ── Filters bar ─────────────────────────────────────────────── */
.filters-bar {
  display: flex; align-items: center; flex-wrap: wrap; gap: 10px;
  padding: 16px 24px 0;
}

.search-wrap {
  position: relative; flex: 1; min-width: 220px; max-width: 380px;
}
.search-icon {
  position: absolute; left: 11px; top: 50%; transform: translateY(-50%);
  color: var(--ink-soft); pointer-events: none;
}
.search-input {
  width: 100%; height: 38px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: var(--card); padding: 0 34px 0 34px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.search-input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.search-input::placeholder { color: #b0b3be; }
.search-clear {
  position: absolute; right: 10px; top: 50%; transform: translateY(-50%);
  border: none; background: none; color: var(--ink-soft); cursor: pointer; display: grid; place-items: center;
  width: 20px; height: 20px; border-radius: 4px; padding: 0;
  transition: background .15s, color .15s;
}
.search-clear:hover { background: rgba(220,38,38,.08); color: #dc2626; }

.chips { display: flex; flex-wrap: wrap; gap: 6px; }
.chip {
  display: flex; align-items: center; gap: 5px;
  padding: 0 12px; height: 34px; border: 1.5px solid var(--line); border-radius: 20px;
  background: var(--card); color: var(--ink-soft);
  font: 500 13px/1 'Inter', sans-serif; cursor: pointer;
  transition: border-color .15s, background .15s, color .15s;
  white-space: nowrap;
}
.chip:hover { border-color: var(--brand); color: var(--brand); }
.chip.active { background: rgba(59,63,224,.1); border-color: var(--brand); color: var(--brand); font-weight: 600; }
.chip-cnt {
  display: inline-flex; align-items: center; justify-content: center;
  background: currentColor; color: #fff; border-radius: 10px;
  font-size: 10px; font-weight: 700; min-width: 18px; height: 18px; padding: 0 5px;
  opacity: .75;
}
.chip.active .chip-cnt { opacity: 1; }

.group-select-wrap {
  display: flex; align-items: center; gap: 7px;
  color: var(--ink-soft);
}
.group-select {
  height: 34px; padding: 0 28px 0 10px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: var(--card); color: var(--ink);
  font: 500 13px/1 'Inter', sans-serif; cursor: pointer; outline: none;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%236b7280' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 8px center;
  transition: border-color .2s var(--ease);
}
.group-select:focus { border-color: var(--brand); }

/* ── Results bar ─────────────────────────────────────────────── */
.results-bar { padding: 10px 24px 0; }
.results-text { font-size: 13px; color: var(--ink-soft); }
.results-text strong { color: var(--ink); font-weight: 600; }
.results-empty { color: #b0b3be; }

/* ── User grid ───────────────────────────────────────────────── */
.users-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  padding: 16px 24px 0;
}

.user-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px 20px 20px;
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  text-align: center;
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.user-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(20,22,60,.11);
}

/* Аватар */
.u-avatar {
  width: 64px; height: 64px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; overflow: hidden; margin-bottom: 4px;
}
.u-avatar img { width: 100%; height: 100%; object-fit: cover; }
.u-initials {
  font: 700 22px/1 'Inter', sans-serif; color: #fff; letter-spacing: -.5px;
  text-shadow: 0 1px 3px rgba(0,0,0,.2);
}

/* ФИО */
.u-name {
  font-size: 14px; font-weight: 600; color: var(--ink); line-height: 1.35;
}

/* Роль / группа */
.u-badge {
  padding: 3px 10px; border-radius: 20px;
  font: 600 11px/1 'Inter', sans-serif;
}

/* Почта */
.u-email {
  display: flex; align-items: center; gap: 5px;
  font-size: 12px; color: var(--ink-soft); word-break: break-all; margin-top: 2px;
}
.u-email svg { flex-shrink: 0; color: var(--ink-soft); }

/* ── Empty state ─────────────────────────────────────────────── */
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  padding: 60px 24px; gap: 10px;
}
.empty-title { font-size: 16px; font-weight: 600; color: var(--ink); margin: 0; }
.empty-sub   { font-size: 13px; color: var(--ink-soft); margin: 0; text-align: center; line-height: 1.6; }
.btn-reset {
  margin-top: 8px; padding: 0 20px; height: 36px; border-radius: var(--radius);
  border: 1.5px solid var(--line); background: var(--card); color: var(--ink);
  font: 500 13px/1 'Inter', sans-serif; cursor: pointer;
  transition: border-color .15s, color .15s;
}
.btn-reset:hover { border-color: var(--brand); color: var(--brand); }

/* ── Pagination ──────────────────────────────────────────────── */
.pagination {
  display: flex; align-items: center; justify-content: center; gap: 4px;
  padding: 24px 24px 0;
}
.pg-btn {
  min-width: 36px; height: 36px; padding: 0 8px;
  border: 1.5px solid var(--line); border-radius: 8px;
  background: var(--card); color: var(--ink-soft);
  font: 500 13px/1 'Inter', sans-serif; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: border-color .15s, background .15s, color .15s;
}
.pg-btn:hover:not(:disabled) { border-color: var(--brand); color: var(--brand); background: rgba(59,63,224,.05); }
.pg-btn.active { background: var(--brand); border-color: var(--brand); color: #fff; font-weight: 700; }
.pg-btn:disabled { opacity: .35; cursor: not-allowed; }
.pg-dots { min-width: 36px; text-align: center; color: var(--ink-soft); font-size: 14px; }

.page-bottom-space { height: 32px; }

/* ── Responsive ──────────────────────────────────────────────── */
@media (max-width: 1280px) { .users-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 900px)  { .users-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 600px) {
  .page-bar, .filters-bar, .results-bar, .users-grid, .pagination {
    padding-left: 16px; padding-right: 16px;
  }
  .search-wrap { max-width: 100%; }
  .users-grid { grid-template-columns: repeat(2, 1fr); gap: 12px; }
  .user-card { padding: 18px 14px 16px; }
  .u-avatar { width: 52px; height: 52px; }
  .u-initials { font-size: 18px; }
}
@media (max-width: 400px) {
  .users-grid { grid-template-columns: 1fr; }
}
</style>
