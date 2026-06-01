<script setup>
// Плашка пустого состояния: крупная иконка + текст.
// Используется внутри раскрытых панелей дашбордов («долгов нет» и т.п.).
defineProps({
  text: { type: String, required: true },
  // icon: 'check' (всё хорошо), 'calendar' (нет пересдач), 'book' (нет дисциплин)
  icon: { type: String, default: 'check' },
})
</script>

<template>
  <div class="empty-plate">
    <div class="empty-plate-icon">
      <!-- check: ничего не висит / всё закрыто -->
      <svg v-if="icon === 'check'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/>
      </svg>
      <!-- calendar: нет пересдач -->
      <svg v-else-if="icon === 'calendar'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/>
      </svg>
      <!-- book: нет дисциплин -->
      <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/>
      </svg>
    </div>
    <p class="empty-plate-text">{{ text }}</p>
  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

.empty-plate {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px 20px;
  text-align: center;
}
.empty-plate-icon {
  width: 56px; height: 56px;
  border-radius: 50%;
  display: grid; place-items: center;
  background: rgba(59,63,224,.08);
  color: #3b3fe0;
}
.empty-plate-icon svg { width: 28px; height: 28px; }
.empty-plate-text {
  margin: 0;
  font: 500 14px/1.5 'Inter', sans-serif;
  color: #6b7280;
}
</style>
