<script setup>
// Кастомный выпадающий список для фильтров дашбордов.
// Полностью стилизован под проект (в отличие от нативного <select>).
// v-model — выбранное значение; пустая строка = «все».
import { ref, computed, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  modelValue:  { type: [String, Number], default: '' },
  options:     { type: Array, default: () => [] },   // [{ value, label }] | [string]
  placeholder: { type: String, default: 'Все' },
  // Поиск внутри списка. true | 'auto' (включается при >5 вариантах).
  searchable:  { type: [Boolean, String], default: false },
})
const emit = defineEmits(['update:modelValue'])

const open   = ref(false)
const root   = ref(null)
const search = ref('')

// Нормализуем опции к { value, label }
const norm = computed(() =>
  props.options.map(o =>
    (o && typeof o === 'object') ? { value: o.value, label: o.label } : { value: o, label: String(o) }
  )
)

const showSearch = computed(() =>
  props.searchable === true || (props.searchable === 'auto' && norm.value.length > 5)
)
const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return norm.value
  return norm.value.filter(o => o.label.toLowerCase().includes(q))
})

const selectedLabel = computed(() => {
  const hit = norm.value.find(o => o.value === props.modelValue)
  return hit ? hit.label : ''
})

function select(value) {
  emit('update:modelValue', value)
  open.value = false
  search.value = ''
}

function toggle() {
  open.value = !open.value
  if (open.value) search.value = ''
}

function onOutside(e) {
  if (root.value && !root.value.contains(e.target)) open.value = false
}
onMounted(() => document.addEventListener('mousedown', onOutside))
onUnmounted(() => document.removeEventListener('mousedown', onOutside))
</script>

<template>
  <div ref="root" class="fsel" :class="{ open }">
    <button type="button" class="fsel-trigger" @click="toggle">
      <span :class="{ placeholder: !selectedLabel }">{{ selectedLabel || placeholder }}</span>
      <svg class="fsel-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><polyline points="6 9 12 15 18 9"/></svg>
    </button>
    <div v-show="open" class="fsel-dropdown">
      <div v-if="showSearch" class="fsel-search-wrap">
        <input class="fsel-search" v-model="search" placeholder="Поиск..." @click.stop @mousedown.stop />
      </div>
      <div class="fsel-scroll">
        <button
          type="button" class="fsel-option"
          :class="{ selected: modelValue === '' }"
          @click="select('')"
        >{{ placeholder }}</button>
        <button
          v-for="o in filtered" :key="o.value"
          type="button" class="fsel-option"
          :class="{ selected: modelValue === o.value }"
          @click="select(o.value)"
        >{{ o.label }}</button>
        <div v-if="!filtered.length" class="fsel-empty">Ничего не найдено</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

.fsel {
  --brand: #3b3fe0;
  --line:  #d7d9e0;
  --ink:   #1a1d24;
  --ink-soft: #6b7280;
  --ease:  cubic-bezier(.2,.7,.2,1);
  position: relative; flex: 1; min-width: 150px;
}
.fsel-trigger {
  display: flex; align-items: center; justify-content: space-between;
  width: 100%; height: 36px; padding: 0 12px;
  border: 1.5px solid var(--line); border-radius: 8px;
  background: #fff; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; text-align: left; gap: 8px;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.fsel-trigger:hover { border-color: #a0a3b1; }
.fsel.open .fsel-trigger { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.fsel-trigger span {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0;
}
.fsel-trigger .placeholder { color: var(--ink-soft); }
.fsel-arrow { width: 14px; height: 14px; color: var(--ink-soft); flex-shrink: 0; transition: transform .2s var(--ease); }
.fsel.open .fsel-arrow { transform: rotate(180deg); }

.fsel-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0; z-index: 100;
  background: #fff; border: 1.5px solid var(--line); border-radius: 8px;
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.14);
  overflow: hidden;
}
.fsel-search-wrap { padding: 6px 6px 4px; border-bottom: 1px solid #eceef2; }
.fsel-search {
  width: 100%; height: 30px; border: 1.5px solid var(--line); border-radius: 6px;
  padding: 0 10px; font: 12.5px/1 'Inter', sans-serif; color: var(--ink);
  outline: none; background: #fff;
}
.fsel-search:focus { border-color: var(--brand); }
.fsel-scroll { max-height: 240px; overflow-y: auto; padding: 4px; }
.fsel-scroll::-webkit-scrollbar { width: 4px; }
.fsel-scroll::-webkit-scrollbar-track { background: transparent; }
.fsel-scroll::-webkit-scrollbar-thumb { background: #c5c8d4; border-radius: 4px; }
.fsel-empty { padding: 9px 12px; font: 12.5px/1 'Inter', sans-serif; color: var(--ink-soft); }
.fsel-option {
  display: block; width: 100%; text-align: left;
  padding: 9px 12px; border: none; border-radius: 6px;
  background: none; font: 13px/1.3 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; transition: background .12s;
}
.fsel-option:hover { background: rgba(59,63,224,.07); }
.fsel-option.selected { color: var(--brand); font-weight: 600; background: rgba(59,63,224,.06); }
</style>
