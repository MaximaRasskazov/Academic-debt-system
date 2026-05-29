<script setup>
// Пересдачи текущего пользователя. Для всех ролей одно и то же:
// GET /api/retakes/my — список пересдач, где он участник
// (студент с долгом или преподаватель в составе).
import { ref, onMounted, computed } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'

const auth = useAuthStore()
const router = useRouter()
const retakes = ref([])
const disciplineMap = ref({})
const loading = ref(true)
const error = ref('')
const filter = ref('upcoming') // upcoming | past | all

onMounted(async () => {
  await loadRetakes()
  loadDisciplines()
})

async function loadRetakes() {
  loading.value = true
  error.value = ''
  try {
    const resp = await retakesApi.listMy()
    // Backend может вернуть массив или {items}. Нормализуем.
    retakes.value = resp.items || resp || []
  } catch (e) {
    error.value =
      e.response?.data?.message || 'Не удалось загрузить пересдачи'
  } finally {
    loading.value = false
  }
}

async function loadDisciplines() {
  try {
    const list = await disciplinesApi.list({ limit: 200 })
    const items = list.items || list
    disciplineMap.value = Object.fromEntries(
      items.map((d) => [d.id, d.name]),
    )
  } catch {}
}

const filteredRetakes = computed(() => {
  const now = new Date()
  return retakes.value
    .filter((r) => {
      const dt = new Date(r.scheduled_at)
      if (filter.value === 'upcoming')
        return dt >= now && r.status !== 'cancelled' && r.status !== 'completed'
      if (filter.value === 'past')
        return r.status === 'completed' || r.status === 'cancelled' || dt < now
      return true
    })
    .sort((a, b) => new Date(a.scheduled_at) - new Date(b.scheduled_at))
})

function statusLabel(s) {
  return {
    scheduled: 'Запланирована',
    in_progress: 'Идёт сейчас',
    completed: 'Завершена',
    cancelled: 'Отменена',
  }[s] || s
}

function formatDateTime(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
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
        <h1>Мои пересдачи</h1>
        <p class="sub" v-if="auth.user">{{ auth.fullName }}</p>
      </div>
      <div class="head-actions">
        <RouterLink to="/debts" class="link">Долги</RouterLink>
        <RouterLink to="/profile" class="link">Профиль</RouterLink>
        <button class="btn-logout" @click="logout">Выйти</button>
      </div>
    </header>

    <div v-if="loading" class="state">Загрузка…</div>

    <div v-else-if="error" class="state error">
      {{ error }}
      <button class="btn-retry" @click="loadRetakes">Попробовать снова</button>
    </div>

    <template v-else>
      <div class="filters">
        <button :class="['chip', filter === 'upcoming' && 'active']"
                @click="filter = 'upcoming'">Предстоящие</button>
        <button :class="['chip', filter === 'past' && 'active']"
                @click="filter = 'past'">Прошедшие</button>
        <button :class="['chip', filter === 'all' && 'active']"
                @click="filter = 'all'">Все ({{ retakes.length }})</button>
      </div>

      <div v-if="filteredRetakes.length === 0" class="state empty">
        Пересдач в этой категории нет.
      </div>

      <ul v-else class="list">
        <li v-for="r in filteredRetakes" :key="r.id" :class="['card', `status-${r.status}`]">
          <div class="when">
            <div class="date-big">{{ formatDateTime(r.scheduled_at) }}</div>
            <div class="duration">{{ r.duration_minutes }} мин · {{ r.kind === 'commission' ? 'Комиссия' : 'Обычная' }}</div>
          </div>
          <div class="info">
            <div class="title">{{ disciplineMap[r.discipline_id] || r.discipline_id }}</div>
            <div class="place">{{ r.building }}, ауд. {{ r.room }}</div>
            <span :class="['badge', `badge-${r.status}`]">{{ statusLabel(r.status) }}</span>
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
  background: none; border: 1.5px solid #d7d9e0;
  border-radius: 8px; padding: 6px 12px; font-size: 13px; cursor: pointer; color: #6b7280;
}
.btn-logout:hover { border-color: #b9bcc8; color: #1a1d24; }

.filters { display: flex; gap: 8px; margin-bottom: 20px; flex-wrap: wrap; }
.chip {
  border: 1.5px solid #d7d9e0; background: #fff;
  border-radius: 999px; padding: 6px 14px; font-size: 13px;
  cursor: pointer; transition: all .15s;
}
.chip:hover { border-color: #b9bcc8; }
.chip.active { background: #3b3fe0; border-color: #3b3fe0; color: #fff; }

.state { text-align: center; padding: 60px 20px; color: #6b7280; font-size: 14px; }
.state.error { color: #d63a51; }
.state.empty { background: #f7f7fb; border-radius: 12px; }
.btn-retry {
  margin-top: 12px; padding: 6px 16px; border-radius: 8px;
  border: 1.5px solid #3b3fe0; background: #fff; color: #3b3fe0; cursor: pointer;
}

.list { list-style: none; padding: 0; margin: 0; display: grid; gap: 12px; }
.card {
  background: #fff; border: 1.5px solid #e5e7eb;
  border-radius: 12px; padding: 16px 20px;
  display: grid; grid-template-columns: 180px 1fr; gap: 20px; align-items: center;
}
.card.status-scheduled { border-left: 4px solid #3b3fe0; }
.card.status-in_progress { border-left: 4px solid #f59e0b; }
.card.status-completed { border-left: 4px solid #2e7d32; opacity: .8; }
.card.status-cancelled { border-left: 4px solid #9ca3af; opacity: .6; }

.when { border-right: 1px solid #e5e7eb; padding-right: 20px; }
.date-big { font-weight: 600; font-size: 15px; margin-bottom: 4px; }
.duration { font-size: 12px; color: #6b7280; }

.info { display: flex; flex-direction: column; gap: 6px; }
.title { font-weight: 600; font-size: 15px; }
.place { font-size: 13px; color: #6b7280; }
.badge {
  align-self: flex-start;
  padding: 2px 8px; border-radius: 6px; font-size: 12px; font-weight: 500;
}
.badge-scheduled { background: #dbeafe; color: #1e40af; }
.badge-in_progress { background: #fef3c7; color: #92400e; }
.badge-completed { background: #d1fae5; color: #065f46; }
.badge-cancelled { background: #e5e7eb; color: #374151; }

@media (max-width: 600px) {
  .card { grid-template-columns: 1fr; }
  .when { border-right: none; border-bottom: 1px solid #e5e7eb; padding-right: 0; padding-bottom: 8px; }
}
</style>
