<script setup>
import { reactive, ref, computed, onUnmounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { getHomeForRole } from '../router'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const view = ref('login')
const loading = ref(false)
const triedSubmit = ref(false)
// Серверная ошибка логина — показывается под кнопкой при неверных
// учётных данных, rate-limit, 5xx и т.д.
const loginError = ref('')

const titles = {
  'login':            'Авторизация',
  'recover-email':    'Восстановление пароля',
  'recover-code':     'Введите код',
  'recover-password': 'Новый пароль',
  'recover-done':     'Готово',
}

// ---- login form
const form = reactive({ email: '', password: '', consent: false })
const canSubmit = computed(() =>
  form.email.trim().length > 0 &&
  form.password.length > 0 &&
  form.consent
)

async function onSubmit() {
  triedSubmit.value = true
  loginError.value = ''
  if (!canSubmit.value) return
  loading.value = true
  try {
    await auth.login(form.email.trim(), form.password)
    // После успешного login отправляем на "домашнюю" страницу роли:
    // student → /debts, teacher → /teacher, dean → /dean. Если в query
    // лежит ?next=... (роутер кладёт туда исходный путь, когда гость
    // тыкается в защищённый маршрут) — возвращаемся туда.
    const next = route.query.next
    router.push(next || getHomeForRole(auth.primaryRole))
  } catch (e) {
    const status = e.response?.status
    const msg = e.response?.data?.message
    if (status === 401) loginError.value = 'Неверный email или пароль'
    else if (status === 429)
      loginError.value =
        'Слишком много попыток входа. Подождите ~15 секунд и попробуйте снова'
    else loginError.value = msg || 'Не удалось войти. Попробуйте позже'
  } finally {
    loading.value = false
  }
}

// ---- recovery flow
const recover = reactive({
  email: '',
  code: ['', '', '', '', '', ''],
  codeError: '',
  error: '',
  newPassword: '',
  confirmPassword: '',
})
const codeRefs = ref([])
const RESEND_SECONDS = 60
const resendIn = ref(0)
let resendTimer = null

const emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const isValidEmail = computed(() => emailRe.test(recover.email.trim()))
const isCodeComplete = computed(() => recover.code.every(d => d !== ''))

const passwordError = computed(() => {
  if (!recover.newPassword && !recover.confirmPassword) return ''
  if (recover.newPassword.length > 0 && recover.newPassword.length < 8)
    return 'Пароль должен быть не короче 8 символов.'
  if (recover.confirmPassword.length > 0 && recover.newPassword !== recover.confirmPassword)
    return 'Пароли не совпадают.'
  return ''
})
const canSavePassword = computed(() =>
  recover.newPassword.length >= 8 &&
  recover.newPassword === recover.confirmPassword
)

function startResendTimer() {
  clearInterval(resendTimer)
  resendIn.value = RESEND_SECONDS
  resendTimer = setInterval(() => {
    resendIn.value--
    if (resendIn.value <= 0) { clearInterval(resendTimer); resendTimer = null }
  }, 1000)
}
function formatTime(s) {
  const m = Math.floor(s / 60), r = s % 60
  return `${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}`
}

function goLogin() {
  view.value = 'login'
  recover.code = ['', '', '', '', '', '']
  recover.codeError = ''
  recover.error = ''
  recover.newPassword = ''
  recover.confirmPassword = ''
  clearInterval(resendTimer); resendTimer = null; resendIn.value = 0
}
function goRecover() {
  view.value = 'recover-email'
  recover.code = ['', '', '', '', '', '']
  recover.codeError = ''
  clearInterval(resendTimer); resendTimer = null; resendIn.value = 0
}

async function sendCode() {
  recover.error = ''
  if (!isValidEmail.value) { recover.error = 'Некорректный email.'; return }
  loading.value = true
  await new Promise(r => setTimeout(r, 700))
  loading.value = false
  view.value = 'recover-code'
  startResendTimer()
  nextTick(() => codeRefs.value[0]?.focus())
}

async function resendCode() {
  if (resendIn.value > 0) return
  loading.value = true
  await new Promise(r => setTimeout(r, 500))
  loading.value = false
  recover.code = ['', '', '', '', '', '']
  recover.codeError = ''
  startResendTimer()
  nextTick(() => codeRefs.value[0]?.focus())
}

async function verifyCode() {
  recover.codeError = ''
  if (!isCodeComplete.value) return
  loading.value = true
  await new Promise(r => setTimeout(r, 700))
  loading.value = false
  view.value = 'recover-password'
}

async function savePassword() {
  if (!canSavePassword.value) return
  loading.value = true
  await new Promise(r => setTimeout(r, 700))
  loading.value = false
  view.value = 'recover-done'
}

function onCodeInput(e, i) {
  const raw = e.target.value.replace(/\D/g, '')
  if (raw.length > 1) { distributeDigits(raw, i); return }
  recover.code[i] = raw
  e.target.value = raw
  recover.codeError = ''
  if (raw && i < 5) codeRefs.value[i + 1]?.focus()
}
function distributeDigits(str, startAt) {
  let idx = startAt
  for (const ch of str) {
    if (idx > 5) break
    recover.code[idx] = ch
    idx++
  }
  nextTick(() => codeRefs.value[Math.min(idx, 5)]?.focus())
}
function onCodeKeydown(e, i) {
  if (e.key === 'Backspace') {
    if (!recover.code[i] && i > 0) {
      e.preventDefault()
      recover.code[i - 1] = ''
      codeRefs.value[i - 1]?.focus()
    }
  } else if (e.key === 'ArrowLeft' && i > 0) {
    e.preventDefault(); codeRefs.value[i - 1]?.focus()
  } else if (e.key === 'ArrowRight' && i < 5) {
    e.preventDefault(); codeRefs.value[i + 1]?.focus()
  }
}
function onCodePaste(e) {
  const text = (e.clipboardData || window.clipboardData).getData('text')
  const digits = text.replace(/\D/g, '').slice(0, 6)
  if (!digits) return
  e.preventDefault()
  recover.code = ['', '', '', '', '', '']
  distributeDigits(digits, 0)
}

onUnmounted(() => { clearInterval(resendTimer) })
</script>

<template>
  <div class="page">
    <div class="card" role="main">

      <!-- ===== Brand side ===== -->
      <aside class="brand" aria-hidden="false">
        <span class="grain" aria-hidden="true"></span>

        <div class="logo" aria-label="Академический Ассистент">
          <div class="logo-mark" aria-hidden="true"></div>
          <div class="logo-text">
            Академический <br> Ассистент
          </div>
        </div>

        <h1 class="brand-headline">
          <span>Начни</span>
          <span>учиться</span>
          <span>прямо сейчас!</span>
        </h1>
      </aside>

      <!-- ===== Form side ===== -->
      <section class="form-panel" aria-labelledby="auth-title">
        <h2 id="auth-title" class="form-title">{{ titles[view] }}</h2>

        <div class="panel-stack">

          <!-- LOGIN -->
          <form v-if="view === 'login'" @submit.prevent="onSubmit" novalidate>
            <div class="field">
              <label for="email">Email</label>
              <input id="email" class="input" type="email" autocomplete="email"
                     v-model="form.email" :disabled="loading" />
            </div>

            <div class="field">
              <div class="field-row">
                <label for="password">Пароль</label>
                <span class="helper">
                  Забыли пароль?
                  <a href="#" @click.prevent="goRecover">Восстановить</a>
                </span>
              </div>
              <input id="password" class="input" type="password" autocomplete="current-password"
                     v-model="form.password" :disabled="loading" />
            </div>

            <label class="check">
              <input type="checkbox" v-model="form.consent" />
              <span class="box" aria-hidden="true">
                <svg viewBox="0 0 16 16"><path d="M3 8.5l3.2 3.2L13 5"/></svg>
              </span>
              <span>Я соглашаюсь на использование cookie согласно
                <a href="#" @click.prevent>политике</a></span>
            </label>

            <p v-if="!form.consent && triedSubmit" class="form-error">Необходимо согласие на использование cookie.</p>
            <p v-if="loginError" class="form-error">{{ loginError }}</p>

            <button class="submit" type="submit" :disabled="!canSubmit || loading">
              {{ loading ? 'Входим…' : 'Войти' }}
            </button>

            <p class="register-hint">
              Нет аккаунта?
              <RouterLink to="/register">Зарегистрироваться</RouterLink>
            </p>
          </form>

          <!-- RECOVER · EMAIL -->
          <form v-else-if="view === 'recover-email'" @submit.prevent="sendCode" novalidate>
            <p class="step-sub">Укажите email, привязанный к аккаунту.<br/>Мы отправим на него код подтверждения.</p>

            <div class="field">
              <label for="recover-email">Email</label>
              <input id="recover-email" class="input" type="email" autocomplete="email"
                     v-model="recover.email" :disabled="loading" placeholder="name@example.com" />
            </div>

            <p v-if="recover.error" class="form-error">{{ recover.error }}</p>

            <button type="button" class="back-link" @click="goLogin">
              <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M10 3L5 8l5 5"/>
              </svg>
              Назад
            </button>

            <button class="submit" type="submit" :disabled="!isValidEmail || loading">
              {{ loading ? 'Отправляем…' : 'Отправить код' }}
            </button>
          </form>

          <!-- RECOVER · CODE -->
          <form v-else-if="view === 'recover-code'" @submit.prevent="verifyCode" novalidate>
            <p class="step-sub">Код отправлен на<br/><strong>{{ recover.email }}</strong></p>

            <div class="field">
              <label>Код подтверждения</label>
              <div class="code-row" @paste="onCodePaste">
                <input
                  v-for="(d, i) in 6"
                  :key="i"
                  :ref="el => codeRefs[i] = el"
                  class="code-cell"
                  :class="{ filled: !!recover.code[i], error: recover.codeError }"
                  type="text"
                  inputmode="numeric"
                  maxlength="1"
                  :value="recover.code[i] || ''"
                  @input="onCodeInput($event, i)"
                  @keydown="onCodeKeydown($event, i)"
                  @focus="$event.target.select()"
                  :disabled="loading"
                  autocomplete="one-time-code"
                />
              </div>
            </div>

            <div class="resend-row">
              <span v-if="resendIn > 0">Отправить повторно через <span class="timer">{{ formatTime(resendIn) }}</span></span>
              <button v-else type="button" @click="resendCode" :disabled="loading">Отправить код ещё раз</button>
              <button type="button" @click="goRecover">Изменить почту</button>
            </div>

            <p v-if="recover.codeError" class="form-error">{{ recover.codeError }}</p>

            <button class="submit" type="submit" :disabled="!isCodeComplete || loading">
              {{ loading ? 'Проверяем…' : 'Подтвердить' }}
            </button>
          </form>

          <!-- RECOVER · NEW PASSWORD -->
          <form v-else-if="view === 'recover-password'" @submit.prevent="savePassword" novalidate>
            <p class="step-sub">Придумайте новый пароль.<br/>Минимум 8 символов.</p>

            <div class="field">
              <label for="new-pass">Новый пароль</label>
              <input id="new-pass" class="input" type="password" autocomplete="new-password"
                     v-model="recover.newPassword" :disabled="loading" />
            </div>

            <div class="field">
              <label for="new-pass-2">Подтверждение пароля</label>
              <input id="new-pass-2" class="input" type="password" autocomplete="new-password"
                     v-model="recover.confirmPassword" :disabled="loading" />
            </div>

            <p v-if="passwordError" class="form-error">{{ passwordError }}</p>

            <button class="submit" type="submit" :disabled="!canSavePassword || loading">
              {{ loading ? 'Сохраняем…' : 'Сохранить пароль' }}
            </button>
          </form>

          <!-- RECOVER · DONE -->
          <div v-else-if="view === 'recover-done'" class="success">
            <div class="success-ring" aria-hidden="true">
              <svg viewBox="0 0 24 24"><path d="M5 12.5l4.2 4.2L19 7"/></svg>
            </div>
            <p>Пароль успешно обновлён.</p>
            <p class="muted">Теперь можно войти в аккаунт с новым паролем.</p>
            <button class="submit" type="button" @click="goLogin" style="margin-top:8px">
              Вернуться к входу
            </button>
          </div>

        </div>
      </section>

    </div>
  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Oswald:wght@500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

/* CSS-переменные на .page — единственный способ работы в scoped-стилях Vue */
.page {
  --bg:        #f3f4f7;
  --card:      #ffffff;
  --ink:       #1a1d24;
  --ink-soft:  #6b7280;
  --line:      #d7d9e0;
  --brand:     #3b3fe0;
  --brand-ink: #2a2e9e;
  --link:      #3b6df0;
  --radius-lg: 10px;
  --shadow:    0 30px 60px -30px rgba(20,22,60,.25), 0 2px 6px rgba(20,22,60,.04);
  --ease:      cubic-bezier(.2,.7,.2,1);

  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: 32px;
  background: var(--bg);
  font-family: 'Inter', system-ui, -apple-system, 'Segoe UI', sans-serif;
  color: var(--ink);
  -webkit-font-smoothing: antialiased;
}

.card {
  width: min(880px, 100%);
  background: var(--card);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow);
  overflow: hidden;
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 500px;
}

/* Brand panel */
.brand {
  position: relative;
  overflow: hidden;
  border-radius: 0 10px 10px 0;
  color: #fff;
  padding: 28px 32px 32px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: space-between;
  isolation: isolate;
  background-color: #1d1462;
  background-image: url('../assets/images/background.svg');
}

.brand .grain {
  position: absolute; inset: 0; z-index: -1;
  opacity: .18; pointer-events: none;
  background-image:
    radial-gradient(rgba(255,255,255,.5) 1px, transparent 1px),
    radial-gradient(rgba(255,255,255,.3) 1px, transparent 1px);
  background-size: 3px 3px, 5px 5px;
  background-position: 0 0, 1px 2px;
  mix-blend-mode: overlay;
}

.logo {
  display: flex;
  align-items: center;
  gap: 12px;
  width: fit-content;
}
.logo-mark {
  width: 28px; height: 48px;
  display: grid; place-items: center;
  background-image: url('../assets/icons/Vector.svg');
  background-position: center;
  background-size: 100%;
  background-repeat: no-repeat;
}
.logo-text {
  font-family: 'Gerhaus', 'Regular';
  font-weight: 500;
  font-size: 20px;
  line-height: 1.10;
  text-transform: uppercase;
  user-select: none;
  text-align: left;
}
.logo-text small {
  display: block;
  font-size: 13px;
  opacity: .95;
}

.brand-headline {
  font-family: 'Gerhaus', 'Regular';
  text-transform: uppercase;
  font-weight: 500;
  letter-spacing: .01em;
  font-size: clamp(25px, 3.2vw, 38px);
  line-height: 1.1;
  user-select: none;
  margin: 0;
  text-align: left;
  color: #fff;
}
.brand-headline span { display: block; }

/* Form panel */
.form-panel {
  padding: 40px 48px;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.form-title {
  font-family: 'Gerhaus', 'Regular';
  text-transform: uppercase;
  color: #3C38B6;
  font-weight: 500;
  letter-spacing: .02em;
  font-size: 22px;
  text-align: center;
  margin: 4px 0 50px;
}

form {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: auto;
  margin-bottom: auto;
  text-align: left;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.field-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
}
.field label {
  font-size: 13px;
  color: var(--ink);
  font-weight: 400;
  text-align: left;
}
.field .helper {
  font-size: 11px;
  color: var(--ink-soft);
}
.field .helper a {
  color: var(--link);
  text-decoration: none;
  margin-left: 4px;
}
.field .helper a:hover { text-decoration: underline; }

.input {
  appearance: none;
  width: 100%;
  height: 38px;
  border: 1.5px solid var(--line);
  border-radius: 10px;
  background: #fff;
  padding: 0 12px;
  font: inherit;
  font-size: 13px;
  color: var(--ink);
  outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input::placeholder { color: #b7b9c2; }
.input:hover { border-color: #b9bcc8; }
.input:focus {
  border-color: var(--brand);
  box-shadow: 0 0 0 4px rgba(59,63,224,.14);
}

/* Checkbox */
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--ink-soft);
  user-select: none;
  cursor: pointer;
  text-align: left;
}
.check input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
.check .box {
  width: 14px; height: 14px;
  border: 1.5px solid #b9bcc8;
  border-radius: 3px;
  background: #fff;
  display: grid; place-items: center;
  transition: all .15s var(--ease);
  flex-shrink: 0;
}
.check .box svg {
  width: 10px; height: 10px;
  stroke: #fff;
  stroke-width: 3;
  fill: none;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-dasharray: 16;
  stroke-dashoffset: 16;
  transition: stroke-dashoffset .25s var(--ease);
}
.check input:checked + .box {
  background: var(--brand);
  border-color: var(--brand);
}
.check input:checked + .box svg { stroke-dashoffset: 0; }
.check a { color: var(--link); text-decoration: none; margin-left: 2px; }
.check a:hover { text-decoration: underline; }

/* Button */
.submit {
  align-self: center;
  min-width: 180px;
  padding: 0 28px;
  height: 40px;
  border: none;
  border-radius: 10px;
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff;
  font: 600 14px/1 'Inter', sans-serif;
  letter-spacing: .01em;
  cursor: pointer;
  box-shadow: 0 10px 22px -10px rgba(91,59,217,.6);
  transition: transform .25s var(--ease), box-shadow .25s var(--ease);
  margin-top: 6px;
}
.submit:hover:not([disabled]):not(:disabled) {
  transform: scale(1.04);
  box-shadow: 0 4px 10px -5px rgba(91,59,217,.7);
}
.submit:active { transform: scale(1.01); }
.submit[disabled],
.submit:disabled { opacity: .5; cursor: not-allowed; transform: none; box-shadow: none; }

.btn-ghost {
  background: transparent;
  color: var(--brand-ink);
  border: 1.5px solid var(--line);
  height: 38px;
  padding: 0 18px;
  border-radius: 10px;
  font: 500 13px/1 'Inter', sans-serif;
  cursor: pointer;
  transition: border-color .2s var(--ease), color .2s var(--ease);
}
.btn-ghost:hover:not([disabled]) {
  border-color: #b9bcc8;
  background: #f7f7fb;
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: none;
  border: none;
  padding: 0;
  color: var(--link);
  font: 500 13px/1 'Inter', sans-serif;
  cursor: pointer;
  align-self: flex-start;
  margin-bottom: 4px;
  transition: color .15s var(--ease), transform .15s var(--ease);
}
.back-link:hover { color: var(--brand-ink); transform: translateX(-2px); }
.back-link svg { width: 14px; height: 14px; }

.step-sub {
  text-align: center;
  font-size: 13px;
  color: var(--ink-soft);
  margin: -10px 0 14px;
  line-height: 1.45;
}
.step-sub strong { color: var(--ink); font-weight: 600; }

/* 6-digit code */
.code-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}
.code-cell {
  flex: 1;
  min-width: 0;
  width: 0;
  height: 48px;
  border: 1.5px solid var(--line);
  border-radius: 10px;
  background: #fff;
  text-align: center;
  font: 600 20px/1 'Inter', sans-serif;
  color: var(--ink);
  outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease), transform .15s var(--ease);
  /* -moz-appearance: textfield; */
}
.code-cell::-webkit-outer-spin-button,
.code-cell::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.code-cell:hover { border-color: #b9bcc8; }
.code-cell:focus {
  border-color: var(--brand);
  box-shadow: 0 0 0 4px rgba(59,63,224,.14);
  transform: translateY(-1px);
}
.code-cell.filled { border-color: var(--brand); color: var(--brand-ink); }
.code-cell.error { border-color: #e0445a; box-shadow: 0 0 0 4px rgba(224,68,90,.12); }

.resend-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--ink-soft);
}
.resend-row .timer { color: var(--ink); font-variant-numeric: tabular-nums; }
.resend-row button {
  background: none; border: none; padding: 0; cursor: pointer;
  color: var(--link); font: 500 12px/1 'Inter', sans-serif;
}
.resend-row button:hover { text-decoration: underline; }
.resend-row button[disabled] { color: #b7b9c2; cursor: not-allowed; text-decoration: none; }

.form-error {
  font-size: 12px;
  color: #d63a51;
  margin-top: -6px;
  text-align: left;
}

.register-hint {
  text-align: center;
  font-size: 12px;
  color: var(--ink-soft);
  margin-top: 4px;
}
.register-hint a {
  color: var(--link);
  text-decoration: none;
}
.register-hint a:hover { text-decoration: underline; }

/* Success */
.success {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 8px 0 4px;
}
.success-ring {
  width: 64px; height: 64px;
  border-radius: 50%;
  display: grid; place-items: center;
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  box-shadow: 0 10px 30px -10px rgba(91,59,217,.6);
  animation: pop .35s cubic-bezier(.2,.7,.2,1) both;
}
.success-ring svg {
  width: 30px; height: 30px;
  stroke: #fff; stroke-width: 3; fill: none;
  stroke-linecap: round; stroke-linejoin: round;
  stroke-dasharray: 30; stroke-dashoffset: 30;
  animation: drawCheck .5s cubic-bezier(.2,.7,.2,1) .15s forwards;
}
@keyframes drawCheck { to { stroke-dashoffset: 0; } }
@keyframes pop { from { transform: scale(.6); opacity: 0; } to { transform: none; opacity: 1; } }
.success p { font-size: 14px; color: var(--ink); margin: 0; max-width: 260px; line-height: 1.45; }
.success p.muted { color: var(--ink-soft); font-size: 12px; }

.panel-stack { position: relative; flex: 1; display: flex; flex-direction: column; }
.panel-stack > form,
.panel-stack > .success { flex: 1; display: flex; flex-direction: column; }

/* Responsive */
@media (max-width: 980px) {
  .page { padding: 24px; }
  .card { min-height: 460px; }
  .form-panel { padding: 36px 36px; }
  .brand { padding: 24px 26px 28px; }
  .brand-headline { font-size: clamp(22px, 3.6vw, 32px); }
}

@media (max-width: 680px) {
  .page { padding: 16px; align-items: flex-start; }
  .card {
    grid-template-columns: 1fr;
    min-height: 0;
    border-radius: 10px;
    max-width: 420px;
  }
  .brand {
    border-radius: 0 0 10px 10px;
    padding: 24px 24px 28px;
    min-height: 140px;
    justify-content: center;
    align-items: center;
    text-align: center;
  }
  .logo { margin: 0 auto; }
  .brand-headline { display: none; }
  .form-panel { padding: 28px 22px 30px; }
  .form-title { margin: 2px 0 22px; font-size: 20px; }
  form { gap: 14px; }
  .submit { height: 40px; }
}

@media (max-width: 360px) {
  .form-panel { padding: 28px 18px 32px; }
}

@media (prefers-reduced-motion: reduce) {
  * { animation: none !important; transition: none !important; }
}
</style>
