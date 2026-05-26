<script setup>
import { ref, onMounted } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'
import { debtsApi } from '../api/debts'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'

const sidebarOpen = ref(false)

const upcomingRetakes = ref([])
const stats = ref([
  { label: 'Мои предметы',       value: '—', accent: '#3b3fe0' },
  { label: 'Активных долгов',    value: '—', accent: '#e63c5a' },
  { label: 'Назначено пересдач', value: '—', accent: '#f59e0b' },
  { label: 'Завершено в месяце', value: '—', accent: '#10b981' },
])

onMounted(async () => {
  const [myDiscRes, debtsRes, retakesRes] = await Promise.allSettled([
    disciplinesApi.myAsTeacher(),
    debtsApi.getByDiscipline(),
    retakesApi.getMy(),
  ])

  const discMap = {}
  const myDiscs = myDiscRes.status === 'fulfilled'
    ? (myDiscRes.value.data.items ?? myDiscRes.value.data ?? [])
    : []
  myDiscs.forEach(d => { discMap[d.id] = d.name || d.code })

  const activeDebts = debtsRes.status === 'fulfilled'
    ? (debtsRes.value.data.total ?? (debtsRes.value.data.items ?? []).length)
    : 0

  let retakeCount = 0, completedMonth = 0
  if (retakesRes.status === 'fulfilled') {
    const items = retakesRes.value.data.items ?? retakesRes.value.data ?? []
    retakeCount = items.filter(r => r.status === 'scheduled' || r.status === 'in_progress').length
    const now = new Date()
    completedMonth = items.filter(r => {
      if (r.status !== 'completed') return false
      const d = new Date(r.completed_at ?? r.updated_at ?? r.created_at)
      return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth()
    }).length
    upcomingRetakes.value = items
      .filter(r => r.status === 'scheduled')
      .slice(0, 10)
      .map(r => {
        const d = new Date(r.scheduled_at)
        return {
          id: r.id,
          subject: discMap[r.discipline_id] || 'Дисциплина',
          day: d.getDate(), month: d.getMonth() + 1, year: d.getFullYear(),
          time: `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`,
          building: r.building, room: r.room,
        }
      })
  }

  stats.value = [
    { label: 'Мои предметы',       value: myDiscs.length, accent: '#3b3fe0' },
    { label: 'Активных долгов',    value: activeDebts,    accent: '#e63c5a' },
    { label: 'Назначено пересдач', value: retakeCount,    accent: '#f59e0b' },
    { label: 'Завершено в месяце', value: completedMonth, accent: '#10b981' },
  ]
})
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
