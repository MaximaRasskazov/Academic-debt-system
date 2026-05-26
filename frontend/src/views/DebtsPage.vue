<script setup>
// Список долгов текущего пользователя.
//
// Для роли student показывает свои долги (GET /api/debts/my).
// Для teacher показываем по его дисциплинам (GET /api/debts/by-discipline).
// Для dean/admin — переадресуем на /dean, где полная панель отчётов.
import { ref, onMounted, computed } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { debtsApi } from '../api/debts'
import { disciplinesApi } from '../api/disciplines'

const auth = useAuthStore()
const router = useRouter()
const debts = ref([])
const disciplineMap = ref({}) // id → {name, code} для рендеринга
const loading = ref(true)
const error = ref('')
const statusFilter = ref('all') // all | open | graded | cancelled

onMounted(async () => {
  // Dean/admin — лучше отправить в свою панель, там и сводки, и фильтры.
  if (auth.isDean || auth.isAdmin) {
    router.replace('/dean')
    return
  }
  await loadDebts()
  await loadDisciplineNames()
})

async function loadDebts() {
  loading.value = true
  error.value = ''
  try {
    if (auth.isStudent) {
      debts.value = await debtsApi.listMy()
    } else if (auth.isTeacher) {
      debts.value = await debtsApi.listByDiscipline()
    } else {
      debts.value = []
    }
  } catch (e) {
    error.value =
      e.response?.data?.message || 'Не удалось загрузить долги'
  } finally {
    loading.value = false
  }
}

// Подтягиваем имена дисциплин — в debt только discipline_id, для UX
// показываем человеко-читаемое имя. Список дисциплин маленький, кешируем.
async function loadDisciplineNames() {
  try {
    const list = await disciplinesApi.list({ limit: 200 })
    const items = list.items || list
    disciplineMap.value = Object.fromEntries(
      items.map((d) => [d.id, { name: d.name, code: d.code }]),
    )
  } catch {
    // имена — украшение, без них тоже норм покажем UUID
  }
}

const filteredDebts = computed(() => {
  if (statusFilter.value === 'all') return debts.value
  return debts.value.filter((d) => d.status === statusFilter.value)
})

const counts = computed(() => ({
  open: debts.value.filter((d) => d.status === 'open').length,
  graded: debts.value.filter((d) => d.status === 'graded').length,
  cancelled: debts.value.filter((d) => d.status === 'cancelled').length,
  total: debts.value.length,
}))

function disciplineLabel(id) {
  const d = disciplineMap.value[id]
  return d ? `${d.name} (${d.code})` : id
}

function statusLabel(s) {
  return { open: 'Открыт', graded: 'Закрыт оценкой', cancelled: 'Отменён' }[s] || s
}

function formatDate(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleDateString('ru-RU')
}

async function logout() {
  await auth.logout()
  router.push('/login')
}
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1>Мои долги</h1>
        <p class="sub" v-if="auth.user">
          {{ auth.fullName }} · {{ auth.user.group_name || auth.user.email }}
        </p>
      </div>
      <div class="head-actions">
        <RouterLink to="/retakes" class="link">Мои пересдачи</RouterLink>
        <RouterLink to="/profile" class="link">Профиль</RouterLink>
        <button class="btn-logout" @click="logout">Выйти</button>
      </div>
    </header>

    <div v-if="loading" class="state">Загрузка…</div>

    <div v-else-if="error" class="state error">
      {{ error }}
      <button class="btn-retry" @click="loadDebts">Попробовать снова</button>
    </div>

    <template v-else>
      <div class="filters">
        <button :class="['chip', statusFilter === 'all' && 'active']"
                @click="statusFilter = 'all'">Все ({{ counts.total }})</button>
        <button :class="['chip', statusFilter === 'open' && 'active']"
                @click="statusFilter = 'open'">Открытые ({{ counts.open }})</button>
        <button :class="['chip', statusFilter === 'graded' && 'active']"
                @click="statusFilter = 'graded'">Закрытые ({{ counts.graded }})</button>
        <button :class="['chip', statusFilter === 'cancelled' && 'active']"
                @click="statusFilter = 'cancelled'">Отменённые ({{ counts.cancelled }})</button>
      </div>

      <div v-if="filteredDebts.length === 0" class="state empty">
        У вас нет долгов в этой категории. Так держать!
      </div>

      <ul v-else class="list">
        <li v-for="d in filteredDebts" :key="d.id" :class="['card', `status-${d.status}`]">
          <div class="card-main">
            <div class="discipline">{{ disciplineLabel(d.discipline_id) }}</div>
            <div class="meta">
              <span :class="['badge', `badge-${d.status}`]">{{ statusLabel(d.status) }}</span>
              <span v-if="d.final_grade" class="grade">Оценка: {{ d.final_grade }}</span>
              <span class="date">Создан: {{ formatDate(d.created_at) }}</span>
              <span v-if="d.graded_at" class="date">Закрыт: {{ formatDate(d.graded_at) }}</span>
            </div>
            <div v-if="d.notes" class="notes">{{ d.notes }}</div>
          </div>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.page {
  max-width: 960px;
  margin: 0 auto;
  padding: 32px 24px;
  font-family: 'Inter', system-ui, sans-serif;
  color: #1a1d24;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 24px;
  gap: 16px;
  flex-wrap: wrap;
}
h1 { font-size: 28px; margin: 0 0 4px; }
.sub { margin: 0; color: #6b7280; font-size: 14px; }
.head-actions { display: flex; gap: 12px; align-items: center; }
.link { color: #3b6df0; text-decoration: none; font-size: 14px; }
.link:hover { text-decoration: underline; }
.btn-logout {
  background: none;
  border: 1.5px solid #d7d9e0;
  border-radius: 8px;
  padding: 6px 12px;
  font-size: 13px;
  cursor: pointer;
  color: #6b7280;
}
.btn-logout:hover { border-color: #b9bcc8; color: #1a1d24; }

.filters {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}
.chip {
  border: 1.5px solid #d7d9e0;
  background: #fff;
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  cursor: pointer;
  transition: all .15s;
}
.chip:hover { border-color: #b9bcc8; }
.chip.active {
  background: #3b3fe0;
  border-color: #3b3fe0;
  color: #fff;
}

.state {
  text-align: center;
  padding: 60px 20px;
  color: #6b7280;
  font-size: 14px;
}
.state.error { color: #d63a51; }
.state.empty { background: #f7f7fb; border-radius: 12px; }
.btn-retry {
  margin-top: 12px;
  display: block;
  margin-left: auto;
  margin-right: auto;
  padding: 6px 16px;
  border-radius: 8px;
  border: 1.5px solid #3b3fe0;
  background: #fff;
  color: #3b3fe0;
  cursor: pointer;
}

.list { list-style: none; padding: 0; margin: 0; display: grid; gap: 12px; }
.card {
  background: #fff;
  border: 1.5px solid #e5e7eb;
  border-radius: 12px;
  padding: 16px 20px;
  display: flex;
  gap: 16px;
  align-items: center;
}
.card.status-graded { border-left: 4px solid #2e7d32; }
.card.status-open { border-left: 4px solid #f59e0b; }
.card.status-cancelled { border-left: 4px solid #9ca3af; opacity: .7; }

.card-main { flex: 1; min-width: 0; }
.discipline {
  font-weight: 600;
  font-size: 15px;
  margin-bottom: 6px;
}
.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 16px;
  align-items: center;
  font-size: 13px;
  color: #6b7280;
}
.badge {
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
}
.badge-open { background: #fef3c7; color: #92400e; }
.badge-graded { background: #d1fae5; color: #065f46; }
.badge-cancelled { background: #e5e7eb; color: #374151; }
.grade { font-weight: 600; color: #2e7d32; }
.notes {
  margin-top: 8px;
  font-size: 13px;
  color: #4b5563;
  white-space: pre-line;
}
</style>
