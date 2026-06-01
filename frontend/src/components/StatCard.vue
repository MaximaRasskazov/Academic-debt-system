<script setup>
// Карточка-метрика для дашбордов (декан / преподаватель / студент).
// Стиль: заголовок капсом с сине-фиолетовым градиентом, крупная цифра
// тем же градиентом, «Подробнее →» снизу справа. Белый фон.
// Заголовок и цифра выровнены по левому краю.
defineProps({
  label:     { type: String, required: true },
  value:     { type: [String, Number], default: '—' },
  clickable: { type: Boolean, default: false },
  active:    { type: Boolean, default: false },
})
defineEmits(['click'])
</script>

<template>
  <div
    class="stat-card"
    :class="{ 'stat-card--clickable': clickable, 'stat-card--active': active }"
    @click="clickable && $emit('click')"
  >
    <div class="stat-title">{{ label }}</div>
    <div class="stat-value">{{ value }}</div>
    <div v-if="clickable" class="stat-more">
      {{ active ? 'Скрыть ↑' : 'Подробнее →' }}
    </div>
  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

.stat-card {
  --brand: #3b3fe0;
  --line:  #d7d9e0;
  --ink-soft: #6b7280;
  --ease:  cubic-bezier(.2,.7,.2,1);

  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  text-align: left;
  min-height: 132px;
  padding: 18px 20px 16px;
  background: #fff;
  border: 1.5px solid transparent;
  border-radius: 14px;
  box-shadow: 0 2px 8px rgba(20,22,60,.07);
  transition: box-shadow .18s var(--ease), transform .18s var(--ease), border-color .18s var(--ease);
}

.stat-card--clickable { cursor: pointer; }
.stat-card--clickable:hover {
  box-shadow: 0 4px 18px rgba(20,22,60,.13);
  transform: translateY(-2px);
}
.stat-card--active {
  border-color: var(--brand);
  box-shadow: 0 0 0 1px var(--brand), 0 4px 18px rgba(59,63,224,.15);
  transform: translateY(-2px);
}

/* Заголовок капсом с сине-фиолетовым градиентом, по левому краю */
.stat-title {
  align-self: flex-start;
  text-align: left;
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-weight: 700;
  font-size: 16px;
  letter-spacing: .04em;
  text-transform: uppercase;
  line-height: 1.15;
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: #3C38B6; /* fallback */
}

/* Крупная цифра тем же градиентом, по левому краю */
.stat-value {
  align-self: flex-start;
  margin-top: auto;
  text-align: left;
  font-family: 'Gerhaus', 'Inter', sans-serif;
  font-weight: 700;
  font-size: 46px;
  line-height: 1;
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: #3b3fe0; /* fallback */
}

.stat-more {
  position: absolute;
  right: 18px;
  bottom: 14px;
  font: 500 12px/1 'Inter', sans-serif;
  color: var(--ink-soft);
}

@media (max-width: 600px) {
  .stat-card { min-height: 112px; padding: 14px 16px 12px; }
  .stat-value { font-size: 38px; }
  .stat-title { font-size: 14px; }
}
</style>
