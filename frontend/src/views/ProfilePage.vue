<script setup>
import { ref, computed, reactive } from 'vue'
import { useAuthStore } from '../stores/auth'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'

const auth = useAuthStore()
const sidebarOpen = ref(false)

const ROLE_LABEL = { STUDENT: 'Студент', TEACHER: 'Преподаватель', DEAN: 'Деканат' }

// ── Avatar ────────────────────────────────────────────────
const avatarInput = ref(null)
const avatarPreview = ref(auth.user?.avatar || null)

function pickAvatar() { avatarInput.value?.click() }

function onAvatarFile(e) {
  const file = e.target.files[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = ev => { avatarPreview.value = ev.target.result }
  reader.readAsDataURL(file)
}

// ── Email ─────────────────────────────────────────────────
const emailEditing = ref(false)
const emailDraft = ref(auth.user?.email || '')

function startEmailEdit() {
  emailDraft.value = auth.user?.email || ''
  emailEditing.value = true
}
function cancelEmailEdit() {
  emailDraft.value = auth.user?.email || ''
  emailEditing.value = false
}

// ── Password ──────────────────────────────────────────────
const pw = reactive({ current: '', next: '', confirm: '' })
const pwError = ref('')
const pwSuccess = ref(false)

function changePassword() {
  pwError.value = ''
  if (!pw.current) { pwError.value = 'Введите текущий пароль'; return }
  if (!pw.next)    { pwError.value = 'Введите новый пароль'; return }
  if (pw.next !== pw.confirm) { pwError.value = 'Пароли не совпадают'; return }
  // TODO: API call
  pw.current = ''; pw.next = ''; pw.confirm = ''
  pwSuccess.value = true
  setTimeout(() => { pwSuccess.value = false }, 2500)
}

// ── Save ──────────────────────────────────────────────────
const saved = ref(false)

function save() {
  if (emailEditing.value && auth.user) {
    auth.user.email = emailDraft.value
    emailEditing.value = false
  }
  if (avatarPreview.value !== (auth.user?.avatar || null) && auth.user) {
    auth.user.avatar = avatarPreview.value
  }
  if (auth.user) localStorage.setItem('user', JSON.stringify(auth.user))
  // TODO: API call
  saved.value = true
  setTimeout(() => { saved.value = false }, 2500)
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
            <button class="avatar-btn" @click="pickAvatar" title="Изменить фото">
              <img v-if="avatarPreview" :src="avatarPreview" class="avatar-img" alt="Фото профиля" />
              <span v-else class="avatar-initials">{{ initials }}</span>
              <div class="avatar-overlay">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                  <path d="M23 19a2 2 0 01-2 2H3a2 2 0 01-2-2V8a2 2 0 012-2h4l2-3h6l2 3h4a2 2 0 012 2z"/>
                  <circle cx="12" cy="13" r="4"/>
                </svg>
              </div>
            </button>
            <input ref="avatarInput" type="file" accept="image/*" hidden @change="onAvatarFile" />
            <div class="hero-meta">
              <div class="hero-name">{{ fullName }}</div>
              <button class="link-btn" @click="pickAvatar">Изменить фото</button>
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
            <template v-if="!emailEditing">
              <span class="email-text">{{ auth.user?.email }}</span>
              <button class="link-btn" @click="startEmailEdit">Изменить</button>
            </template>
            <template v-else>
              <input
                class="field-input email-input"
                v-model="emailDraft"
                type="email"
                placeholder="Новый email"
                autofocus
              />
              <button class="link-btn link-cancel" @click="cancelEmailEdit">Отмена</button>
            </template>
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
              <button class="btn-outline" @click="changePassword">Изменить</button>
            </div>
          </div>
          <p v-if="pwError" class="pw-error">{{ pwError }}</p>
          <p v-if="pwSuccess" class="pw-success">Пароль успешно изменён</p>

          <!-- ── Save ── -->
          <div class="save-row">
            <button class="btn-primary btn-save" @click="save">
              <template v-if="saved">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M20 6L9 17l-5-5"/>
                </svg>
                Сохранено
              </template>
              <template v-else>Сохранить</template>
            </button>
          </div>

        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

.profile-root {
  --bg:        #f3f4f7;
  --card:      #ffffff;
  --ink:       #1a1d24;
  --ink-soft:  #6b7280;
  --line:      #d7d9e0;
  --brand:     #3b3fe0;
  --brand-ink: #2a2e9e;
  --radius:    10px;
  --shadow:    0 2px 8px rgba(20,22,60,.07);
  --ease:      cubic-bezier(.2,.7,.2,1);

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

.link-btn {
  background: none; border: none; padding: 0;
  font: 500 13px/1 'Inter', sans-serif; color: var(--ink-soft);
  cursor: pointer; text-align: left;
  transition: color .15s;
}
.link-btn:hover { color: var(--brand); }
.link-cancel:hover { color: #dc2626; }

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

.email-input {
  flex: 1;
  min-width: 200px;
  height: 38px;
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

/* ── Save ── */
.save-row {
  display: flex;
  justify-content: center;
  margin-top: 28px;
  padding-top: 24px;
  border-top: 1px solid var(--line);
}

.btn-primary {
  display: flex; align-items: center; gap: 8px;
  height: 44px; padding: 0 40px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 14px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 4px 14px -6px rgba(91,59,217,.6);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-primary:hover { transform: scale(1.03); box-shadow: 0 4px 10px -4px rgba(91,59,217,.75); }
.btn-save svg { width: 16px; height: 16px; }

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
