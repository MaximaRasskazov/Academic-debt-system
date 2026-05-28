<script setup>
import { watch } from 'vue'
import { useAuthStore } from './stores/auth'
import { useNotificationsStore } from './stores/notifications'
import ToastContainer from './components/ToastContainer.vue'

const auth    = useAuthStore()
const notify  = useNotificationsStore()

// Реагируем на изменение токена — подключаем/отключаем WS и грузим
// начальный список уведомлений. immediate: true чтобы сработало сразу
// при старте, если токен уже есть в localStorage.
watch(
  () => auth.token,
  (token) => {
    if (token) {
      notify.loadInitial()
      notify.connect(token)
    } else {
      notify.disconnect()
    }
  },
  { immediate: true },
)
</script>

<template>
  <RouterView />
  <ToastContainer />
</template>
