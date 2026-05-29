<script setup>
defineProps({
  retakes: { type: Array, default: () => [] },
})

function formatDayLabel(r) {
  return new Date(r.year, r.month, r.day)
    .toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
}
</script>

<template>
  <div class="section-card">
    <h2 class="section-title">Ближайшие пересдачи</h2>
    <ul class="notif-list">
      <li v-for="r in retakes" :key="r.id" class="notif-item">
        <div class="notif-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 12h6M9 16h6M7 4h10a2 2 0 012 2v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6a2 2 0 012-2z"/>
          </svg>
        </div>
        <div class="notif-body">
          <div class="notif-subject">{{ r.subject }}</div>
          <div class="notif-details">
            <div class="notif-detail-row">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="3" y1="10" x2="21" y2="10"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="16" y1="2" x2="16" y2="6"/></svg>
              {{ formatDayLabel(r) }}
            </div>
            <div class="notif-detail-row">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/></svg>
              {{ r.time }}
            </div>
            <div class="notif-detail-row">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7z"/><circle cx="12" cy="9" r="2.5"/></svg>
              корп. {{ r.building }}, ауд. {{ r.room }}
            </div>
          </div>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px; margin-bottom: 20px;
}
.section-card:last-child { margin-bottom: 0; }
.section-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0 0 20px;
}

.notif-list { list-style: none; padding: 0; margin: 0; }
.notif-item {
  display: flex; align-items: flex-start; gap: 12px;
  padding: 12px 0; border-bottom: 1px solid var(--line);
}
.notif-item:last-child { border-bottom: none; padding-bottom: 0; }
.notif-item:first-child { padding-top: 0; }

.notif-icon {
  width: 38px; height: 38px; border-radius: 50%;
  display: grid; place-items: center; flex-shrink: 0;
  background: rgba(59,63,224,.1); color: var(--brand);
}
.notif-icon svg { width: 18px; height: 18px; }

.notif-body { flex: 1; min-width: 0; }
.notif-subject {
  font-size: 13px; font-weight: 600; color: var(--ink);
  margin-bottom: 8px; line-height: 1.3; text-align: left;
}
.notif-details { display: flex; flex-direction: column; gap: 5px; }
.notif-detail-row {
  display: flex; align-items: center; gap: 7px;
  font-size: 12px; color: var(--ink-soft); text-align: left;
}
.notif-detail-row svg { width: 13px; height: 13px; flex-shrink: 0; }
</style>
