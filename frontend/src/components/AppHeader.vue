<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useNotificationsStore } from '../stores/notifications'
import { kindLabel } from '../stores/notifications'
import ProfilePanel from './ProfilePanel.vue'

defineEmits(['open-sidebar'])

const auth    = useAuthStore()
const notify  = useNotificationsStore()
const profileOpen = ref(false)
const bellOpen    = ref(false)
const bellRef     = ref(null)

const roleLabel = computed(() => ({
  admin:   'Администратор',
  dean:    'Деканат',
  teacher: 'Преподаватель',
  student: 'Студент',
}[auth.primaryRole] ?? ''))

function onOutside(e) {
  if (bellRef.value && !bellRef.value.contains(e.target)) bellOpen.value = false
}
onMounted(()   => document.addEventListener('mousedown', onOutside))
onUnmounted(() => document.removeEventListener('mousedown', onOutside))

function timeAgo(iso) {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60_000)
  if (m < 1)  return 'только что'
  if (m < 60) return `${m} мин назад`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} ч назад`
  return `${Math.floor(h / 24)} д назад`
}
</script>

<template>
  <header class="header">
    <div class="header-left">
      <button class="burger" @click="$emit('open-sidebar')" aria-label="Открыть меню">
        <span /><span /><span />
      </button>
      <span class="app-name">Академический<br>Ассистент</span>
    </div>
    <div class="header-right">
      <div class="user-meta">
        <span class="user-name">{{ auth.user?.last_name }} {{ auth.user?.first_name }} {{ auth.user?.middle_name || '' }}</span>
        <span class="user-role">{{ roleLabel }}</span>
      </div>

      <!-- ── Колокольчик уведомлений ── -->
      <div class="bell-wrap" ref="bellRef">
        <button
          class="bell-btn"
          :aria-label="`Уведомления${notify.unreadCount ? ', непрочитанных: ' + notify.unreadCount : ''}`"
          @click="bellOpen = !bellOpen"
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/>
            <path d="M13.73 21a2 2 0 01-3.46 0"/>
          </svg>
          <span v-if="notify.unreadCount > 0" class="bell-badge" aria-hidden="true">
            {{ notify.unreadCount > 99 ? '99+' : notify.unreadCount }}
          </span>
        </button>

        <div v-if="bellOpen" class="bell-dropdown" role="menu">
          <div class="bell-header">
            <span>Уведомления</span>
          </div>
          <p v-if="notify.items.length === 0" class="bell-empty">Нет уведомлений</p>
          <button
            v-for="n in notify.items.slice(0, 10)"
            :key="n.id"
            class="bell-item"
            :class="{ 'bell-item--unread': !n.read_at }"
            role="menuitem"
            @click="notify.markRead(n.id); bellOpen = false"
          >
            <span class="bell-item__dot" />
            <span class="bell-item__body">
              <span class="bell-item__text">{{ kindLabel(n.kind) }}</span>
              <span class="bell-item__time">{{ timeAgo(n.created_at) }}</span>
            </span>
          </button>
        </div>
      </div>

      <button class="avatar" @click="profileOpen = true" aria-label="Открыть профиль">
        <img v-if="auth.user?.avatar" :src="auth.user.avatar" alt="Фото профиля" />
        <span v-else>{{ auth.user?.first_name?.[0] }}{{ auth.user?.last_name?.[0] }}</span>
      </button>
    </div>
  </header>

  <ProfilePanel :open="profileOpen" @close="profileOpen = false" />
</template>

<style scoped>
.header {
  height: 64px;
  background: var(--card);
  border-bottom: 1px solid var(--line);
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 24px;
  position: sticky; top: 0; z-index: 10; flex-shrink: 0;
}
.header-left { display: flex; align-items: center; gap: 16px; }
.burger {
  background: none; border: none; cursor: pointer; padding: 8px;
  display: flex; flex-direction: column; justify-content: center; gap: 3px;
}
.burger span { display: block; width: 22px; height: 2.5px; background: #3C38B6; }
.app-name {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 13px; font-weight: 700; color: #3C38B6;
  text-transform: uppercase; letter-spacing: .04em; line-height: 1.25;
}
.header-right { display: flex; align-items: center; gap: 15px; }
.user-meta { display: flex; flex-direction: column; align-items: flex-end; }
.user-name { font-size: 14px; font-weight: 500; white-space: nowrap; }
.user-role { font-size: 12px; color: var(--ink-soft); }

/* ── Bell ── */
.bell-wrap { position: relative; }
.bell-btn {
  position: relative;
  background: none; border: none; cursor: pointer;
  padding: 8px; color: var(--ink-soft, #9097a8);
  display: grid; place-items: center;
  border-radius: 8px; transition: background .15s, color .15s;
}
.bell-btn:hover { background: var(--line, #f0f1f5); color: #3b3fe0; }
.bell-badge {
  position: absolute; top: 2px; right: 2px;
  min-width: 17px; height: 17px; padding: 0 4px;
  border-radius: 9px;
  background: #e63c5a; color: #fff;
  font-size: 10px; font-weight: 700; line-height: 17px; text-align: center;
}
.bell-dropdown {
  position: absolute; top: calc(100% + 8px); right: 0;
  width: 300px;
  background: var(--card, #fff);
  border: 1px solid var(--line, #e8eaf0);
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0,0,0,.12);
  z-index: 100;
  overflow: hidden;
}
.bell-header {
  padding: 12px 16px 8px;
  font-size: 13px; font-weight: 600; color: var(--ink-soft, #9097a8);
  border-bottom: 1px solid var(--line, #e8eaf0);
}
.bell-empty {
  padding: 24px 16px; text-align: center;
  font-size: 13px; color: var(--ink-soft, #9097a8);
  margin: 0;
}
.bell-item {
  width: 100%; display: flex; align-items: flex-start; gap: 10px;
  padding: 10px 16px; border: none; background: none;
  cursor: pointer; text-align: left;
  transition: background .12s;
  border-bottom: 1px solid var(--line, #f0f1f5);
}
.bell-item:last-child { border-bottom: none; }
.bell-item:hover { background: var(--line, #f5f6fb); }
.bell-item--unread { background: rgba(59, 63, 224, 0.04); }
.bell-item__dot {
  width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; margin-top: 5px;
  background: transparent;
}
.bell-item--unread .bell-item__dot { background: #3b3fe0; }
.bell-item__body { display: flex; flex-direction: column; gap: 2px; flex: 1; }
.bell-item__text { font-size: 13px; color: var(--ink, #1a1d2e); line-height: 1.4; }
.bell-item__time { font-size: 11px; color: var(--ink-soft, #9097a8); }

/* ── Avatar ── */
.avatar {
  width: 40px; height: 40px; border-radius: 50%; flex-shrink: 0;
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  display: grid; place-items: center;
  color: #fff; font-weight: 700; font-size: 14px;
  border: none; cursor: pointer; overflow: hidden;
  transition: transform .2s, box-shadow .2s;
}
.avatar:hover { transform: scale(1.07); box-shadow: 0 4px 12px -4px rgba(91,59,217,.5); }
.avatar img { width: 100%; height: 100%; object-fit: cover; }

@media (max-width: 600px) {
  .user-meta { display: none; }
}
</style>
