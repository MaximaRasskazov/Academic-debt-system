import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useToastsStore = defineStore('toasts', () => {
  const items = ref([])

  function push(message, kind = 'info') {
    const id = `${Date.now()}-${Math.random()}`
    items.value.push({ id, message, kind })
    setTimeout(() => remove(id), 3000)
    return id
  }

  function remove(id) {
    const i = items.value.findIndex((t) => t.id === id)
    if (i !== -1) items.value.splice(i, 1)
  }

  return { items, push, remove }
})
