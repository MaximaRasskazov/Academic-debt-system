<script setup>
import { useToastsStore } from '../stores/toasts'
const toasts = useToastsStore()
</script>

<template>
  <Teleport to="body">
    <div class="toast-list" aria-live="polite" aria-atomic="false">
      <TransitionGroup name="toast">
        <div
          v-for="t in toasts.items"
          :key="t.id"
          class="toast"
          :class="`toast--${t.kind}`"
          role="alert"
          @click="toasts.remove(t.id)"
        >
          <span class="toast__dot" />
          <span class="toast__msg">{{ t.message }}</span>
          <button class="toast__close" aria-label="Закрыть">×</button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-list {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 10px;
  pointer-events: none;
}
.toast {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 280px;
  max-width: 380px;
  padding: 12px 16px;
  border-radius: 10px;
  background: var(--card, #fff);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.13);
  font-size: 14px;
  cursor: pointer;
  pointer-events: all;
  border-left: 4px solid #3b3fe0;
}
.toast--success { border-left-color: #059669; }
.toast--error   { border-left-color: #dc2626; }
.toast--warning { border-left-color: #d97706; }
.toast__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #3b3fe0;
  flex-shrink: 0;
}
.toast--success .toast__dot { background: #059669; }
.toast--error   .toast__dot { background: #dc2626; }
.toast--warning .toast__dot { background: #d97706; }
.toast__msg   { flex: 1; }
.toast__close {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 18px;
  color: var(--ink-soft, #9097a8);
  line-height: 1;
  padding: 0;
}
.toast-enter-active { transition: all 0.25s ease; }
.toast-leave-active { transition: all 0.2s ease; }
.toast-enter-from   { opacity: 0; transform: translateX(40px); }
.toast-leave-to     { opacity: 0; transform: translateX(40px); }
</style>
