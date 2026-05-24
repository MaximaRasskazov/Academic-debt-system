<script setup>
import { ref } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import CalendarWidget from '../components/CalendarWidget.vue'
import UpcomingRetakes from '../components/UpcomingRetakes.vue'

const sidebarOpen = ref(false)

const upcomingRetakes = [
  { id: 1, subject: 'Математический анализ', day: 25, month: 4, year: 2026, time: '10:00', building: '1', room: '204' },
  { id: 2, subject: 'Физика',                day: 27, month: 4, year: 2026, time: '14:00', building: '2', room: '308' },
  { id: 3, subject: 'Линейная алгебра',      day: 30, month: 4, year: 2026, time: '09:00', building: '3', room: '112' },
]

const stats = [
  { label: 'Мои предметы',       value: 5,  accent: '#3b3fe0' },
  { label: 'Активных долгов',    value: 23, accent: '#e63c5a' },
  { label: 'Назначено пересдач', value: 4,  accent: '#f59e0b' },
  { label: 'Завершено в месяце', value: 8,  accent: '#10b981' },
]
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
