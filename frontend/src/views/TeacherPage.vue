<script setup>
import { ref, onMounted, computed } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'
import { debtsApi } from '../api/debts'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'

const sidebarOpen = ref(false)

// Сырьё с бэка
const myDisciplines = ref([])
const myDebts = ref([])
const myRetakes = ref([])
const disciplineMap = ref({})
const loading = ref(true)

onMounted(async () => {
  loading.value = true
  try {
    // Параллельно — три независимых запроса.
    const [discs, debts, retakes] = await Promise.all([
      disciplinesApi.myAsTeacher(),
      debtsApi.listByDiscipline(),
      retakesApi.listMy(),
    ])
    myDisciplines.value = discs.items || discs || []
    myDebts.value = debts.items || debts || []
    myRetakes.value = retakes.items || retakes || []

    // Имена дисциплин для UpcomingRetakes — нужен полный справочник,
    // т.к. retake может быть и по чужой дисциплине (комиссия).
    const all = await disciplinesApi.list({ limit: 200 })
    const items = all.items || all
    disciplineMap.value = Object.fromEntries(items.map((d) => [d.id, d.name]))
  } finally {
    loading.value = false
  }
})

// Карточки наверху — счётчики по реальным данным.
const stats = computed(() => [
  { label: 'Мои предметы', value: myDisciplines.value.length, accent: '#3b3fe0' },
  {
    label: 'Активных долгов',
    value: myDebts.value.filter((d) => d.status === 'open').length,
    accent: '#e63c5a',
  },
  {
    label: 'Назначено пересдач',
    value: myRetakes.value.filter((r) => r.status === 'scheduled').length,
    accent: '#f59e0b',
  },
  {
    label: 'Завершено в месяце',
    value: completedThisMonth(),
    accent: '#10b981',
  },
])

function completedThisMonth() {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth(), 1)
  return myRetakes.value.filter((r) => {
    if (r.status !== 'completed') return false
    const dt = new Date(r.completed_at || r.scheduled_at)
    return dt >= start
  }).length
}

// Адаптация retake → формат компонента UpcomingRetakes (day/month/year/time).
const upcomingRetakes = computed(() =>
  myRetakes.value
    .filter((r) => r.status === 'scheduled')
    .sort((a, b) => new Date(a.scheduled_at) - new Date(b.scheduled_at))
    .slice(0, 5)
    .map((r) => {
      const dt = new Date(r.scheduled_at)
      return {
        id: r.id,
        subject: disciplineMap.value[r.discipline_id] || 'Дисциплина',
        day: dt.getDate(),
        month: dt.getMonth() + 1,
        year: dt.getFullYear(),
        time: dt.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }),
        building: r.building,
        room: r.room,
      }
    }),
)
</script>

<template>
  <div class="teacher-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-teacher">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">

        <section class="col-left">
          <div class="stats-grid">
            <div v-for="s in stats" :key="s.label" class="stat-card">
              <div class="stat-accent" :style="{ background: s.accent }" />
              <div class="stat-value">{{ s.value }}</div>
              <div class="stat-label">{{ s.label }}</div>
            </div>
          </div>
        </section>

        <section class="col-right">
          <CalendarWidget :retakes="upcomingRetakes" />
          <UpcomingRetakes :retakes="upcomingRetakes" />
        </section>

      </main>
    </div>

  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

.teacher-root {
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

.page-teacher {
  min-height: 100dvh;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

.main {
  flex: 1;
  display: grid;
  grid-template-columns: 7fr 3fr;
  gap: 24px;
  padding: 24px;
  align-items: start;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.stat-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 20px 20px 20px 24px;
  position: relative; overflow: hidden;
}
.stat-accent { position: absolute; left: 0; top: 0; bottom: 0; width: 4px; }
.stat-value { font-size: 34px; font-weight: 700; line-height: 1; margin-bottom: 8px; }
.stat-label { font-size: 12px; color: var(--ink-soft); line-height: 1.4; }

@media (max-width: 1280px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 960px) {
  .main { grid-template-columns: 1fr; }
  .col-right { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
}
@media (max-width: 600px) {
  .main { padding: 16px; gap: 16px; }
  .stats-grid { grid-template-columns: 1fr 1fr; }
  .col-right { grid-template-columns: 1fr; }
}
</style>
