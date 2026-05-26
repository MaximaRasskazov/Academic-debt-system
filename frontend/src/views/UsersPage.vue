<script setup>
import { ref, computed, watch } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'

const sidebarOpen = ref(false)

/* ─── Mock data (48 пользователей) ──────────────────────────── */
const ALL_USERS = [
  /* Деканат */
  { id: 1,  role: 'DEAN',    lastName: 'Смирнова',    firstName: 'Татьяна',    middleName: 'Викторовна',    group: null,     email: 'smirnova@university.ru',    avatar: null },
  { id: 2,  role: 'DEAN',    lastName: 'Крылов',      firstName: 'Игорь',      middleName: 'Петрович',      group: null,     email: 'krylov@university.ru',      avatar: null },
  /* Преподаватели */
  { id: 3,  role: 'TEACHER', lastName: 'Иванов',      firstName: 'Иван',       middleName: 'Иванович',      group: null,     email: 'ivanov@university.ru',      avatar: null },
  { id: 4,  role: 'TEACHER', lastName: 'Волков',      firstName: 'Владимир',   middleName: 'Иванович',      group: null,     email: 'volkov@university.ru',      avatar: null },
  { id: 5,  role: 'TEACHER', lastName: 'Белов',       firstName: 'Алексей',    middleName: 'Николаевич',    group: null,     email: 'belov@university.ru',       avatar: null },
  { id: 6,  role: 'TEACHER', lastName: 'Смирнов',     firstName: 'Сергей',     middleName: 'Сергеевич',     group: null,     email: 'smirnov.s@university.ru',   avatar: null },
  { id: 7,  role: 'TEACHER', lastName: 'Кузнецов',    firstName: 'Андрей',     middleName: 'Владимирович',  group: null,     email: 'kuznetsov@university.ru',   avatar: null },
  { id: 8,  role: 'TEACHER', lastName: 'Попов',       firstName: 'Николай',    middleName: 'Михайлович',    group: null,     email: 'popov@university.ru',       avatar: null },
  { id: 9,  role: 'TEACHER', lastName: 'Орлова',      firstName: 'Елена',      middleName: 'Дмитриевна',    group: null,     email: 'orlova@university.ru',      avatar: null },
  { id: 10, role: 'TEACHER', lastName: 'Захаров',     firstName: 'Михаил',     middleName: 'Алексеевич',    group: null,     email: 'zakharov.m@university.ru',  avatar: null },
  /* Студенты — ИВТ-21 */
  { id: 11, role: 'STUDENT', lastName: 'Петров',      firstName: 'Алексей',    middleName: 'Сергеевич',     group: 'ИВТ-21', email: 'petrov@student.ru',         avatar: null },
  { id: 12, role: 'STUDENT', lastName: 'Сидорова',    firstName: 'Мария',      middleName: 'Ивановна',      group: 'ИВТ-21', email: 'sidorova@student.ru',       avatar: null },
  { id: 13, role: 'STUDENT', lastName: 'Козлов',      firstName: 'Дмитрий',    middleName: 'Александрович', group: 'ИВТ-21', email: 'kozlov@student.ru',         avatar: null },
  { id: 14, role: 'STUDENT', lastName: 'Новикова',    firstName: 'Анна',       middleName: 'Петровна',      group: 'ИВТ-21', email: 'novikova@student.ru',       avatar: null },
  { id: 15, role: 'STUDENT', lastName: 'Морозов',     firstName: 'Антон',      middleName: 'Викторович',    group: 'ИВТ-21', email: 'morozov@student.ru',        avatar: null },
  { id: 16, role: 'STUDENT', lastName: 'Волкова',     firstName: 'Ирина',      middleName: 'Сергеевна',     group: 'ИВТ-21', email: 'volkova@student.ru',        avatar: null },
  { id: 17, role: 'STUDENT', lastName: 'Соколов',     firstName: 'Денис',      middleName: 'Олегович',      group: 'ИВТ-21', email: 'sokolov@student.ru',        avatar: null },
  { id: 18, role: 'STUDENT', lastName: 'Герасимов',   firstName: 'Евгений',    middleName: 'Андреевич',     group: 'ИВТ-21', email: 'gerasimov@student.ru',      avatar: null },
  /* Студенты — ИВТ-22 */
  { id: 19, role: 'STUDENT', lastName: 'Захаров',     firstName: 'Павел',      middleName: 'Алексеевич',    group: 'ИВТ-22', email: 'zakharov@student.ru',       avatar: null },
  { id: 20, role: 'STUDENT', lastName: 'Чернова',     firstName: 'Ольга',      middleName: 'Сергеевна',     group: 'ИВТ-22', email: 'chernova@student.ru',       avatar: null },
  { id: 21, role: 'STUDENT', lastName: 'Орлов',       firstName: 'Максим',     middleName: 'Дмитриевич',    group: 'ИВТ-22', email: 'orlov@student.ru',          avatar: null },
  { id: 22, role: 'STUDENT', lastName: 'Крылова',     firstName: 'Виктория',   middleName: 'Олеговна',      group: 'ИВТ-22', email: 'krylova@student.ru',        avatar: null },
  { id: 23, role: 'STUDENT', lastName: 'Тихонова',    firstName: 'Юлия',       middleName: 'Андреевна',     group: 'ИВТ-22', email: 'tikhonova@student.ru',      avatar: null },
  { id: 24, role: 'STUDENT', lastName: 'Абрамов',     firstName: 'Илья',       middleName: 'Николаевич',    group: 'ИВТ-22', email: 'abramov@student.ru',        avatar: null },
  { id: 25, role: 'STUDENT', lastName: 'Васильева',   firstName: 'Светлана',   middleName: 'Олеговна',      group: 'ИВТ-22', email: 'vasilieva@student.ru',      avatar: null },
  { id: 26, role: 'STUDENT', lastName: 'Калинина',    firstName: 'Алина',      middleName: 'Романовна',     group: 'ИВТ-22', email: 'kalinina@student.ru',       avatar: null },
  /* Студенты — ИВТ-23 */
  { id: 27, role: 'STUDENT', lastName: 'Михайлова',   firstName: 'Екатерина',  middleName: 'Юрьевна',       group: 'ИВТ-23', email: 'mikhailova@student.ru',     avatar: null },
  { id: 28, role: 'STUDENT', lastName: 'Орешников',   firstName: 'Борис',      middleName: 'Александрович', group: 'ИВТ-23', email: 'oreshnikov@student.ru',     avatar: null },
  { id: 29, role: 'STUDENT', lastName: 'Лебедев',     firstName: 'Артём',      middleName: 'Сергеевич',     group: 'ИВТ-23', email: 'lebedev@student.ru',        avatar: null },
  { id: 30, role: 'STUDENT', lastName: 'Морозова',    firstName: 'Алина',      middleName: 'Петровна',      group: 'ИВТ-23', email: 'morozova@student.ru',       avatar: null },
  { id: 31, role: 'STUDENT', lastName: 'Соколова',    firstName: 'Дарья',      middleName: 'Денисовна',     group: 'ИВТ-23', email: 'sokolova@student.ru',       avatar: null },
  { id: 32, role: 'STUDENT', lastName: 'Никитин',     firstName: 'Роман',      middleName: 'Игоревич',      group: 'ИВТ-23', email: 'nikitin@student.ru',        avatar: null },
  { id: 33, role: 'STUDENT', lastName: 'Щербаков',    firstName: 'Николай',    middleName: 'Иванович',      group: 'ИВТ-23', email: 'shcherbakov@student.ru',    avatar: null },
  /* Студенты — ФИЗ-21 */
  { id: 34, role: 'STUDENT', lastName: 'Семёнов',     firstName: 'Кирилл',     middleName: 'Андреевич',     group: 'ФИЗ-21', email: 'semenov@student.ru',        avatar: null },
  { id: 35, role: 'STUDENT', lastName: 'Голубева',    firstName: 'Мария',      middleName: 'Константиновна',group: 'ФИЗ-21', email: 'golubeva@student.ru',       avatar: null },
  { id: 36, role: 'STUDENT', lastName: 'Зайцев',      firstName: 'Никита',     middleName: 'Леонидович',    group: 'ФИЗ-21', email: 'zaitsev@student.ru',        avatar: null },
  { id: 37, role: 'STUDENT', lastName: 'Виноградов',  firstName: 'Степан',     middleName: 'Евгеньевич',    group: 'ФИЗ-21', email: 'vinogradov@student.ru',     avatar: null },
  { id: 38, role: 'STUDENT', lastName: 'Ковалёва',    firstName: 'Полина',     middleName: 'Андреевна',     group: 'ФИЗ-21', email: 'kovaleva@student.ru',       avatar: null },
  { id: 39, role: 'STUDENT', lastName: 'Медведев',    firstName: 'Данил',      middleName: 'Сергеевич',     group: 'ФИЗ-21', email: 'medvedev@student.ru',       avatar: null },
  /* Студенты — ФИЗ-22 */
  { id: 40, role: 'STUDENT', lastName: 'Федоров',     firstName: 'Игорь',      middleName: 'Борисович',     group: 'ФИЗ-22', email: 'fedorov@student.ru',        avatar: null },
  { id: 41, role: 'STUDENT', lastName: 'Лебедева',    firstName: 'Надежда',    middleName: 'Сергеевна',     group: 'ФИЗ-22', email: 'lebedeva@student.ru',       avatar: null },
  { id: 42, role: 'STUDENT', lastName: 'Крылов',      firstName: 'Виктор',     middleName: 'Олегович',      group: 'ФИЗ-22', email: 'krylov.v@student.ru',       avatar: null },
  /* Студенты — ЭКО-22 */
  { id: 43, role: 'STUDENT', lastName: 'Степанова',   firstName: 'Анастасия',  middleName: 'Михайловна',    group: 'ЭКО-22', email: 'stepanova@student.ru',      avatar: null },
  { id: 44, role: 'STUDENT', lastName: 'Андреев',     firstName: 'Глеб',       middleName: 'Викторович',    group: 'ЭКО-22', email: 'andreev@student.ru',        avatar: null },
  { id: 45, role: 'STUDENT', lastName: 'Романова',    firstName: 'Ксения',     middleName: 'Олеговна',      group: 'ЭКО-22', email: 'romanova@student.ru',       avatar: null },
  { id: 46, role: 'STUDENT', lastName: 'Ильин',       firstName: 'Тимур',      middleName: 'Павлович',      group: 'ЭКО-22', email: 'ilyin@student.ru',          avatar: null },
  { id: 47, role: 'STUDENT', lastName: 'Борисова',    firstName: 'Валерия',    middleName: 'Дмитриевна',    group: 'ЭКО-22', email: 'borisova@student.ru',       avatar: null },
  { id: 48, role: 'STUDENT', lastName: 'Кузьмин',     firstName: 'Евгений',    middleName: 'Сергеевич',     group: 'ЭКО-22', email: 'kuzmin@student.ru',         avatar: null },
]

/* ─── Фильтры ────────────────────────────────────────────────── */
const PAGE_SIZE = 20

const searchQuery  = ref('')
const filterRole   = ref('all')
const filterGroup  = ref('')
const currentPage  = ref(1)

/* Сброс страницы при смене фильтров */
watch([searchQuery, filterRole, filterGroup], () => { currentPage.value = 1 })

const allGroups = computed(() =>
  [...new Set(ALL_USERS.filter(u => u.group).map(u => u.group))].sort()
)

const filteredUsers = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  return ALL_USERS.filter(u => {
    const name  = `${u.lastName} ${u.firstName} ${u.middleName}`.toLowerCase()
    const email = u.email.toLowerCase()
    const matchSearch = !q || name.includes(q) || email.includes(q) || (u.group?.toLowerCase().includes(q))
    const matchRole  = filterRole.value === 'all'    || u.role === filterRole.value
    const matchGroup = !filterGroup.value             || u.group === filterGroup.value
    return matchSearch && matchRole && matchGroup
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredUsers.value.length / PAGE_SIZE)))

const paginatedUsers = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return filteredUsers.value.slice(start, start + PAGE_SIZE)
})

const rangeStart = computed(() => (currentPage.value - 1) * PAGE_SIZE + 1)
const rangeEnd   = computed(() => Math.min(currentPage.value * PAGE_SIZE, filteredUsers.value.length))

/* Умная пагинация — показываем не больше 7 кнопок */
const visiblePages = computed(() => {
  const total = totalPages.value
  const cur   = currentPage.value
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)

  const pages = [1]
  if (cur > 3) pages.push('…')
  for (let i = Math.max(2, cur - 1); i <= Math.min(total - 1, cur + 1); i++) pages.push(i)
  if (cur < total - 2) pages.push('…')
  pages.push(total)
  return pages
})

function goPage(p) {
  if (typeof p !== 'number') return
  currentPage.value = Math.max(1, Math.min(totalPages.value, p))
}

/* ─── Helpers ────────────────────────────────────────────────── */
function fullName(u) { return `${u.lastName} ${u.firstName} ${u.middleName}` }

function roleLabel(u) {
  if (u.role === 'DEAN')    return 'Деканат'
  if (u.role === 'TEACHER') return 'Преподаватель'
  return u.group || 'Студент'
}

const ROLE_STYLE = {
  DEAN:    { bg: 'rgba(217,119,6,.1)',   color: '#b45309' },
  TEACHER: { bg: 'rgba(139,61,240,.1)',  color: '#7c22d6' },
  STUDENT: { bg: 'rgba(59,63,224,.1)',   color: '#3b3fe0' },
}

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

function initials(u) { return `${u.lastName[0]}${u.firstName[0]}` }

/* Счётчики по ролям для отображения под фильтром */
const counts = computed(() => ({
  all:     ALL_USERS.length,
  STUDENT: ALL_USERS.filter(u => u.role === 'STUDENT').length,
  TEACHER: ALL_USERS.filter(u => u.role === 'TEACHER').length,
  DEAN:    ALL_USERS.filter(u => u.role === 'DEAN').length,
}))
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
          <span class="total-chip">{{ counts.all }} в системе</span>
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
          <button class="chip" :class="{ active: filterRole === 'all' }" @click="filterRole = 'all'">
            Все <span class="chip-cnt">{{ counts.all }}</span>
          </button>
          <button class="chip" :class="{ active: filterRole === 'STUDENT' }" @click="filterRole = 'STUDENT'; filterGroup = ''">
            Студенты <span class="chip-cnt">{{ counts.STUDENT }}</span>
          </button>
          <button class="chip" :class="{ active: filterRole === 'TEACHER' }" @click="filterRole = 'TEACHER'; filterGroup = ''">
            Преподаватели <span class="chip-cnt">{{ counts.TEACHER }}</span>
          </button>
          <button class="chip" :class="{ active: filterRole === 'DEAN' }" @click="filterRole = 'DEAN'; filterGroup = ''">
            Деканат <span class="chip-cnt">{{ counts.DEAN }}</span>
          </button>
        </div>

        <!-- Группа (только когда фильтр = студенты или все) -->
        <div v-if="filterRole !== 'TEACHER' && filterRole !== 'DEAN'" class="group-select-wrap">
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
        <span v-if="filteredUsers.length > 0" class="results-text">
          Показано&nbsp;<strong>{{ rangeStart }}–{{ rangeEnd }}</strong>&nbsp;из&nbsp;<strong>{{ filteredUsers.length }}</strong>
        </span>
        <span v-else class="results-text results-empty">Никого не найдено</span>
      </div>

      <!-- ── Сетка карточек ── -->
      <div class="users-grid" v-if="paginatedUsers.length">
        <div
          v-for="u in paginatedUsers"
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
            :style="{ background: ROLE_STYLE[u.role].bg, color: ROLE_STYLE[u.role].color }"
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
      <div v-else class="empty-state">
        <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="#d7d9e0" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
        </svg>
        <p class="empty-title">Никого не найдено</p>
        <p class="empty-sub">Попробуйте изменить запрос или сбросить фильтры</p>
        <button class="btn-reset" @click="searchQuery = ''; filterRole = 'all'; filterGroup = ''">Сбросить фильтры</button>
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
