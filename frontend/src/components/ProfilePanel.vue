<script setup>
import { ref, computed, reactive } from 'vue'
import { useAuthStore } from '../stores/auth'
import { authApi } from '../api/auth'

defineProps({ open: Boolean })
defineEmits(['close'])

const auth = useAuthStore()

const ROLE_LABEL = { STUDENT: 'Студент', TEACHER: 'Преподаватель', DEAN: 'Деканат' }

// ── Password ──────────────────────────────────────────────
const pw = reactive({ current: '', next: '', confirm: '' })
const pwError = ref('')
const pwSuccess = ref(false)
const pwLoading = ref(false)

async function changePassword() {
  pwError.value = ''
  pwSuccess.value = false
  if (!pw.current)             { pwError.value = 'Введите текущий пароль'; return }
  if (pw.next.length < 8)      { pwError.value = 'Минимум 8 символов'; return }
  if (pw.next !== pw.confirm)  { pwError.value = 'Пароли не совпадают'; return }
  pwLoading.value = true
  try {
    await authApi.changePassword(pw.current, pw.next)
    pw.current = ''; pw.next = ''; pw.confirm = ''
    pwSuccess.value = true
    setTimeout(() => { pwSuccess.value = false }, 3000)
  } catch (e) {
    const msg = e.response?.data?.message || e.response?.data?.error
    pwError.value = msg || 'Неверный текущий пароль'
  } finally {
    pwLoading.value = false
  }
}

// ── Computed ──────────────────────────────────────────────
const initials = computed(() => {
  const f = auth.user?.firstName?.[0] ?? ''
  const l = auth.user?.lastName?.[0] ?? ''
  return (f + l).toUpperCase()
})

const fullName = computed(() =>
  [auth.user?.lastName, auth.user?.firstName, auth.user?.middleName].filter(Boolean).join(' ')
)
</script>

<template>
  <Teleport to="body">
    <Transition name="panel">
      <div v-if="open" class="panel-root">

        <!-- Overlay -->
        <div class="panel-overlay" @click="$emit('close')" />

        <!-- Panel -->
        <div class="panel">

          <!-- Head -->
          <div class="panel-head">
            <span class="panel-title">Профиль</span>
            <button class="panel-close" @click="$emit('close')" aria-label="Закрыть">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                <path d="M18 6L6 18M6 6l12 12"/>
              </svg>
            </button>
          </div>

          <!-- Scrollable body -->
          <div class="panel-body">

            <!-- Hero -->
            <div class="hero">
              <div class="avatar-display">
                <span class="avatar-initials">{{ initials }}</span>
              </div>
              <div class="hero-name">{{ fullName }}</div>
              <span class="role-chip">{{ ROLE_LABEL[auth.role] }}</span>
            </div>

            <div class="divider" />

            <!-- Email -->
            <div class="section-label">Почта</div>

            <div class="email-row">
              <span class="email-text">{{ auth.user?.email }}</span>
            </div>

            <div class="divider" />

            <!-- Password -->
            <div class="section-label">Изменить пароль</div>

            <div class="field-group">
              <label class="field-label">Текущий пароль</label>
              <input class="field-input" type="password" v-model="pw.current" placeholder="••••••" />
            </div>
            <div class="field-group">
              <label class="field-label">Новый пароль</label>
              <input class="field-input" type="password" v-model="pw.next" placeholder="••••••" />
            </div>
            <div class="field-group">
              <label class="field-label">Повторите пароль</label>
              <input class="field-input" type="password" v-model="pw.confirm" placeholder="••••••" />
            </div>

            <p v-if="pwError" class="pw-error">{{ pwError }}</p>
            <p v-if="pwSuccess" class="pw-success">Пароль успешно изменён</p>

            <button class="btn-outline btn-full" :disabled="pwLoading" @click="changePassword">
              {{ pwLoading ? 'Сохранение…' : 'Изменить пароль' }}
            </button>

          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* ── Variables (inherited from parent or set here) ── */
.panel-root {
  --card:      #ffffff;
  --ink:       #1a1d24;
  --ink-soft:  #6b7280;
  --line:      #d7d9e0;
  --bg:        #f3f4f7;
  --brand:     #3b3fe0;
  --brand-ink: #2a2e9e;
  --radius:    10px;
  --ease:      cubic-bezier(.2,.7,.2,1);
  font-family: 'Inter', system-ui, sans-serif;
}

/* ── Layout ── */
.panel-root {
  position: fixed; inset: 0; z-index: 300;
  display: flex; justify-content: flex-end;
}

.panel-overlay {
  position: absolute; inset: 0;
  background: rgba(10,12,30,.4);
  backdrop-filter: blur(2px);
  -webkit-backdrop-filter: blur(2px);
}

.panel {
  position: relative; z-index: 1;
  width: 380px; max-width: 100vw;
  height: 100%;
  background: var(--card);
  box-shadow: -8px 0 40px rgba(20,22,60,.16);
  display: flex; flex-direction: column;
  border-radius: 16px 0 0 16px;
  overflow: hidden;
}

/* ── Head ── */
.panel-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 20px 20px 16px;
  border-bottom: 1px solid var(--line);
  flex-shrink: 0;
}
.panel-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 700; color: #3C38B6;
}
.panel-close {
  width: 32px; height: 32px; border-radius: 8px;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s, color .15s;
}
.panel-close:hover { background: var(--bg); color: var(--ink); }
.panel-close svg { width: 16px; height: 16px; }

/* ── Body ── */
.panel-body {
  flex: 1; overflow-y: auto; overflow-x: hidden; padding: 24px 20px;
  display: flex; flex-direction: column; gap: 12px;
}
.panel-body::-webkit-scrollbar { width: 4px; }
.panel-body::-webkit-scrollbar-track { background: transparent; }
.panel-body::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }

/* ── Hero ── */
.hero {
  display: flex; flex-direction: column; align-items: center;
  gap: 8px; padding-bottom: 4px;
}

.avatar-display {
  width: 88px; height: 88px; border-radius: 50%; flex-shrink: 0;
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  display: grid; place-items: center;
}
.avatar-initials {
  font: 700 28px/1 'Inter', sans-serif; color: #fff; pointer-events: none;
}

.hero-name {
  font: 700 16px/1.3 'Inter', sans-serif;
  color: var(--ink); text-align: center;
}

.role-chip {
  padding: 3px 12px; border-radius: 20px;
  background: rgba(59,63,224,.1); color: var(--brand-ink);
  font: 600 12px/1.4 'Inter', sans-serif;
}


/* ── Divider ── */
.divider { border: none; border-top: 1px solid var(--line); margin: 4px 0; }

/* ── Section label ── */
.section-label {
  font: 600 12px/1 'Inter', sans-serif;
  color: var(--ink-soft);
  text-transform: uppercase;
  letter-spacing: .05em;
  margin-bottom: 2px;
}

/* ── Fields ── */
.field-group { display: flex; flex-direction: column; gap: 5px; }
.field-label { font: 500 12px/1 'Inter', sans-serif; color: var(--ink-soft); }

.field-input {
  appearance: none; height: 40px; width: 100%; box-sizing: border-box;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.field-input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.field-input::placeholder { color: #b7b9c2; }

/* ── Email row ── */
.email-row {
  display: flex; align-items: center; gap: 10px; min-width: 0;
}
.email-text {
  flex: 1; font: 13px/1 'Inter', sans-serif; color: var(--ink);
  min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

/* ── Password ── */
.pw-error  { font: 12px/1.4 'Inter', sans-serif; color: #dc2626; margin: 0; }
.pw-success { font: 12px/1.4 'Inter', sans-serif; color: #059669; margin: 0; }

/* ── Buttons ── */
.btn-full { width: 100%; box-sizing: border-box; }

.btn-outline {
  height: 44px; padding: 0 18px;
  background: #fff;
  border: 1.5px solid #c5c7d4; border-radius: var(--radius);
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink);
  cursor: pointer; white-space: nowrap;
  box-shadow: 0 1px 4px rgba(20,22,60,.08);
  transition: border-color .15s, color .15s, background .15s, box-shadow .15s;
}
.btn-outline:hover {
  border-color: var(--brand); color: var(--brand);
  background: rgba(59,63,224,.04);
  box-shadow: 0 2px 10px rgba(59,63,224,.14);
}


/* ── Transition ── */
.panel-enter-active .panel-overlay,
.panel-leave-active .panel-overlay {
  transition: opacity .3s var(--ease);
}
.panel-enter-from .panel-overlay,
.panel-leave-to .panel-overlay { opacity: 0; }

.panel-enter-active .panel,
.panel-leave-active .panel {
  transition: transform .32s var(--ease);
}
.panel-enter-from .panel,
.panel-leave-to .panel { transform: translateX(100%); }
</style>
