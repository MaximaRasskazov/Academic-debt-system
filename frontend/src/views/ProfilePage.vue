<script setup>
import { ref, computed, reactive } from 'vue'
import { useAuthStore } from '../stores/auth'
import { authApi } from '../api/auth'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'

const auth = useAuthStore()
const sidebarOpen = ref(false)

const ROLE_LABEL = { STUDENT: 'Студент', TEACHER: 'Преподаватель', DEAN: 'Деканат' }

// ── Password ──────────────────────────────────────────────
const pw = reactive({ current: '', next: '', confirm: '' })
const pwError = ref('')
const pwSuccess = ref(false)
const pwLoading = ref(false)

async function changePassword() {
  pwError.value = ''
  pwSuccess.value = false
  if (!pw.current)               { pwError.value = 'Введите текущий пароль'; return }
  if (pw.next.length < 8)        { pwError.value = 'Минимум 8 символов'; return }
  if (pw.next !== pw.confirm)    { pwError.value = 'Пароли не совпадают'; return }
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
  <div class="profile-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-wrap">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">
        <div class="profile-card">

          <!-- Avatar + Name -->
          <div class="hero-section">
            <div class="avatar-btn">
              <span class="avatar-initials">{{ initials }}</span>
            </div>
            <div class="hero-meta">
              <div class="hero-name">{{ fullName }}</div>
              <span class="role-label">{{ ROLE_LABEL[auth.role] ?? auth.role }}</span>
            </div>
          </div>

          <div class="section-divider" />

          <!-- ── Info fields: Student ── -->
          <div v-if="auth.isStudent" class="fields-grid">
            <div class="field-group">
              <label class="field-label">ФИО</label>
              <input class="field-input field-readonly" :value="fullName" readonly />
            </div>
            <div class="field-group">
              <label class="field-label">Группа</label>
              <input class="field-input field-readonly" :value="auth.user?.group || '—'" readonly />
            </div>
            <div class="field-group">
              <label class="field-label">Курс</label>
              <input class="field-input field-readonly" :value="auth.user?.course || '—'" readonly />
            </div>
          </div>

          <!-- ── Info fields: Dean / Teacher ── -->
          <div v-else class="fields-grid">
            <div class="field-group">
              <label class="field-label">ФИО</label>
              <input class="field-input field-readonly" :value="fullName" readonly />
            </div>
            <div class="field-group">
              <label class="field-label">Должность</label>
              <input class="field-input field-readonly" :value="ROLE_LABEL[auth.role]" readonly />
            </div>
          </div>

          <!-- ── Email ── -->
          <div class="section-title">Почта</div>
          <div class="email-row">
            <div class="email-icon-wrap">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <rect x="2" y="4" width="20" height="16" rx="2"/>
                <path d="M2 7l10 7 10-7"/>
              </svg>
            </div>
            <span class="email-text">{{ auth.user?.email }}</span>
          </div>

          <div class="section-divider" />

          <!-- ── Password ── -->
          <div class="section-title">Изменить пароль</div>
          <div class="pw-row">
            <div class="field-group">
              <label class="field-label">Текущий пароль</label>
              <input class="field-input" type="password" v-model="pw.current" placeholder="••••••" />
            </div>
            <div class="field-group">
              <label class="field-label">Новый пароль</label>
              <input class="field-input" type="password" v-model="pw.next" placeholder="••••••" />
            </div>
            <div class="field-group">
              <label class="field-label">Повтор пароля</label>
              <input class="field-input" type="password" v-model="pw.confirm" placeholder="••••••" />
            </div>
            <div class="pw-action">
              <label class="field-label">&nbsp;</label>
              <button class="btn-outline" :disabled="pwLoading" @click="changePassword">
                {{ pwLoading ? 'Сохранение…' : 'Изменить' }}
              </button>
            </div>
          </div>
          <p v-if="pwError" class="pw-error">{{ pwError }}</p>
          <p v-if="pwSuccess" class="pw-success">Пароль успешно изменён</p>


        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>

*, *::before, *::after { box-sizing: border-box; }

.profile-root {
  min-height: 100dvh;
  font-family: 'Inter', system-ui, sans-serif;
  color: var(--ink);
  -webkit-font-smoothing: antialiased;
}

.page-wrap {
  min-height: 100dvh;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

.main {
  flex: 1;
  padding: 32px 24px;
  display: flex;
  justify-content: center;
  align-items: flex-start;
}

/* ── Profile card ── */
.profile-card {
  width: 100%;
  max-width: 740px;
  background: var(--card);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  border: 1px solid var(--line);
  padding: 32px;
}

/* ── Hero ── */
.hero-section {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 28px;
}

.avatar-btn {
  position: relative;
  width: 88px; height: 88px; border-radius: 50%; flex-shrink: 0;
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  border: none; cursor: pointer; overflow: hidden; padding: 0;
  display: grid; place-items: center;
}
.avatar-img { width: 100%; height: 100%; object-fit: cover; display: block; }
.avatar-initials {
  font: 700 28px/1 'Inter', sans-serif; color: #fff;
  pointer-events: none;
}
.avatar-overlay {
  position: absolute; inset: 0;
  background: rgba(10,12,30,.45);
  display: grid; place-items: center;
  opacity: 0; transition: opacity .2s var(--ease);
}
.avatar-btn:hover .avatar-overlay { opacity: 1; }
.avatar-overlay svg { width: 24px; height: 24px; stroke: #fff; }

.hero-meta { display: flex; flex-direction: column; gap: 6px; }
.hero-name {
  font: 700 20px/1.2 'Inter', sans-serif;
  color: var(--ink);
}

.role-label {
  font: 500 13px/1 'Inter', sans-serif;
  color: var(--ink-soft);
}

/* ── Divider ── */
.section-divider {
  border: none; border-top: 1px solid var(--line); margin: 0 0 24px;
}

/* ── Fields grid ── */
.fields-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px 24px;
  margin-bottom: 28px;
}

.field-group { display: flex; flex-direction: column; gap: 6px; }
.field-label {
  font: 500 12px/1 'Inter', sans-serif;
  color: var(--ink-soft);
  text-transform: none;
}

.field-input {
  appearance: none;
  height: 44px; width: 100%;
  border: 1.5px solid var(--line);
  border-radius: var(--radius);
  background: #fff;
  padding: 0 14px;
  font: 14px/1 'Inter', sans-serif;
  color: var(--ink);
  outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.field-input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.1); }
.field-input::placeholder { color: #b7b9c2; }

.field-readonly {
  background: #f3f4f7;
  color: var(--ink-soft);
  cursor: default;
}
.field-readonly:focus { border-color: var(--line); box-shadow: none; }

/* ── Section title ── */
.section-title {
  font: 700 14px/1 'Inter', sans-serif;
  color: var(--ink);
  margin-bottom: 14px;
}

/* ── Email ── */
.email-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.email-icon-wrap {
  width: 36px; height: 36px; border-radius: 10px;
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  display: grid; place-items: center; flex-shrink: 0;
}
.email-icon-wrap svg { width: 16px; height: 16px; stroke: #fff; }

.email-text {
  font: 14px/1 'Inter', sans-serif;
  color: var(--ink);
  flex: 1;
}

/* ── Password ── */
.pw-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr auto;
  gap: 16px;
  align-items: end;
  margin-bottom: 8px;
}

.pw-action { display: flex; flex-direction: column; gap: 6px; }

.btn-outline {
  height: 44px; padding: 0 18px;
  background: none; border: 1.5px solid var(--line); border-radius: var(--radius);
  font: 500 13px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; white-space: nowrap;
  transition: border-color .15s, color .15s, background .15s;
}
.btn-outline:hover { border-color: var(--brand); color: var(--brand); background: rgba(59,63,224,.04); }

.pw-error {
  font: 13px/1 'Inter', sans-serif;
  color: #dc2626;
  margin: 0 0 16px;
}
.pw-success {
  font: 13px/1 'Inter', sans-serif;
  color: #059669;
  margin: 0 0 16px;
}

/* ── Responsive ── */
@media (max-width: 640px) {
  .main { padding: 16px; }
  .profile-card { padding: 20px; }
  .fields-grid { grid-template-columns: 1fr; }
  .pw-row { grid-template-columns: 1fr; }
  .pw-action { display: none; }
  .hero-name { font-size: 17px; }
}
</style>
