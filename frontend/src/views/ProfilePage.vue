<script setup>
// Профиль пользователя. Backend сейчас не поддерживает смену email/пароля/
// аватара (нет соответствующих endpoint'ов), поэтому страница read-only.
// Если в будущем добавятся PATCH /api/users/me — переделать на форму.
//
// Студенту тут же показываем кнопку "Подать заявку на роль преподавателя"
// — POST /api/teacher-requests с motivation. Список своих заявок ниже.
import { ref, computed, onMounted } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { teacherRequestsApi } from '../api/teacherRequests'

const auth = useAuthStore()
const router = useRouter()

// teacher-request: только для студентов
const myRequests = ref([])
const requestsLoading = ref(false)
const requestsError = ref('')
const motivation = ref('')
const submitting = ref(false)
const submitError = ref('')
const submitSuccess = ref(false)

onMounted(async () => {
  if (auth.isStudent) {
    await loadMyRequests()
  }
})

async function loadMyRequests() {
  requestsLoading.value = true
  requestsError.value = ''
  try {
    const resp = await teacherRequestsApi.listMy()
    myRequests.value = resp.items || resp || []
  } catch (e) {
    requestsError.value =
      e.response?.data?.message || 'Не удалось загрузить заявки'
  } finally {
    requestsLoading.value = false
  }
}

const hasPendingRequest = computed(() =>
  myRequests.value.some((r) => r.status === 'pending'),
)

async function submitTeacherRequest() {
  submitError.value = ''
  submitSuccess.value = false
  submitting.value = true
  try {
    await teacherRequestsApi.create(motivation.value.trim() || undefined)
    motivation.value = ''
    submitSuccess.value = true
    await loadMyRequests()
  } catch (e) {
    if (e.response?.status === 409) {
      submitError.value = 'У вас уже есть активная заявка'
    } else {
      submitError.value =
        e.response?.data?.message || 'Не удалось отправить заявку'
    }
  } finally {
    submitting.value = false
  }
}

const initials = computed(() => {
  const f = auth.user?.first_name?.[0] ?? ''
  const l = auth.user?.last_name?.[0] ?? ''
  return (f + l).toUpperCase()
})

const roleLabel = computed(() => {
  return {
    admin: 'Администратор',
    dean: 'Деканат',
    teacher: 'Преподаватель',
    student: 'Студент',
  }[auth.primaryRole] || '—'
})

function statusLabel(s) {
  return {
    pending: 'На рассмотрении',
    approved: 'Одобрена',
    rejected: 'Отклонена',
  }[s] || s
}

function formatDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

async function logout() {
  await auth.logout()
  router.push('/login')
}

function goHome() {
  router.push('/')
}
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1>Профиль</h1>
      </div>
      <div class="head-actions">
        <button class="link" @click="goHome">← На главную</button>
        <button class="btn-logout" @click="logout">Выйти</button>
      </div>
    </header>

    <div v-if="!auth.user" class="state">Профиль не загружен</div>

    <template v-else>
      <section class="card">
        <div class="hero">
          <div class="avatar">{{ initials || '?' }}</div>
          <div class="hero-info">
            <div class="name">{{ auth.fullName }}</div>
            <div class="role-tag">{{ roleLabel }}</div>
          </div>
        </div>

        <div class="fields">
          <div class="field">
            <label>Email</label>
            <div class="value">{{ auth.user.email }}</div>
          </div>
          <div v-if="auth.user.middle_name" class="field">
            <label>Отчество</label>
            <div class="value">{{ auth.user.middle_name }}</div>
          </div>
          <div v-if="auth.user.group_name" class="field">
            <label>Группа</label>
            <div class="value">{{ auth.user.group_name }}</div>
          </div>
          <div class="field">
            <label>Роли</label>
            <div class="value">
              <span v-for="r in auth.roles" :key="r.id" class="role-chip">
                {{ r.name }}
              </span>
            </div>
          </div>
        </div>
      </section>

      <!-- Студенту даём подать заявку на роль преподавателя -->
      <section v-if="auth.isStudent" class="card">
        <h2>Стать преподавателем</h2>
        <p class="hint">
          Если вы хотите вести занятия и принимать пересдачи —
          подайте заявку. Деканат рассмотрит её и при одобрении
          ваша роль будет расширена.
        </p>

        <form v-if="!hasPendingRequest" @submit.prevent="submitTeacherRequest">
          <label class="lbl">Мотивация (необязательно)</label>
          <textarea
            v-model="motivation"
            class="textarea"
            rows="3"
            placeholder="Кратко напишите, почему хотите получить роль..."
            :disabled="submitting"
          />
          <p v-if="submitError" class="form-error">{{ submitError }}</p>
          <p v-if="submitSuccess" class="form-success">Заявка отправлена</p>
          <button class="btn-primary" :disabled="submitting">
            {{ submitting ? 'Отправляем…' : 'Подать заявку' }}
          </button>
        </form>

        <p v-else class="info-box">
          У вас уже есть активная заявка. Дождитесь решения деканата.
        </p>

        <div v-if="!requestsLoading && myRequests.length > 0" class="history">
          <h3>История заявок</h3>
          <ul class="list">
            <li v-for="r in myRequests" :key="r.id" class="req-item">
              <span :class="['badge', `badge-${r.status}`]">{{ statusLabel(r.status) }}</span>
              <span class="req-date">{{ formatDate(r.created_at) }}</span>
              <span v-if="r.review_reason" class="req-reason">
                Причина: {{ r.review_reason }}
              </span>
            </li>
          </ul>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.page {
  max-width: 740px;
  margin: 0 auto;
  padding: 32px 24px;
  font-family: 'Inter', system-ui, sans-serif;
  color: #1a1d24;
}
.head {
  display: flex; justify-content: space-between; align-items: flex-end;
  margin-bottom: 24px; gap: 16px; flex-wrap: wrap;
}
h1 { font-size: 28px; margin: 0; }
.head-actions { display: flex; gap: 12px; align-items: center; }
.link {
  background: none; border: none; color: #3b6df0;
  font-size: 14px; cursor: pointer; padding: 0;
}
.link:hover { text-decoration: underline; }
.btn-logout {
  background: none; border: 1.5px solid #d7d9e0;
  border-radius: 8px; padding: 6px 12px; font-size: 13px;
  cursor: pointer; color: #6b7280;
}
.btn-logout:hover { border-color: #b9bcc8; color: #1a1d24; }

.state { text-align: center; padding: 60px; color: #6b7280; }

.card {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 24px;
  margin-bottom: 16px;
  box-shadow: 0 2px 8px rgba(20,22,60,.05);
}
.card h2 { margin: 0 0 8px; font-size: 18px; }
.card h3 { margin: 16px 0 8px; font-size: 14px; color: #6b7280; }

.hero { display: flex; gap: 16px; align-items: center; margin-bottom: 20px; }
.avatar {
  width: 64px; height: 64px; border-radius: 50%;
  background: linear-gradient(135deg, #2b5cff, #8b3df0);
  color: #fff; font: 600 22px/1 'Inter', sans-serif;
  display: grid; place-items: center; flex-shrink: 0;
}
.hero-info .name { font-weight: 600; font-size: 18px; }
.role-tag {
  display: inline-block; margin-top: 6px;
  background: #e0e7ff; color: #3730a3;
  padding: 2px 10px; border-radius: 999px; font-size: 12px; font-weight: 500;
}

.fields { display: grid; gap: 12px; }
.field { display: flex; gap: 12px; align-items: baseline; padding: 8px 0; border-top: 1px solid #f3f4f6; }
.field:first-child { border-top: none; }
.field label { width: 110px; font-size: 13px; color: #6b7280; flex-shrink: 0; }
.field .value { font-size: 14px; color: #1a1d24; flex: 1; }
.role-chip {
  display: inline-block; padding: 2px 8px; border-radius: 6px;
  background: #f3f4f6; color: #374151; font-size: 12px;
  margin-right: 6px;
}

.hint { color: #6b7280; font-size: 13px; line-height: 1.5; margin: 0 0 12px; }
.lbl { display: block; font-size: 12px; color: #6b7280; margin-bottom: 4px; }
.textarea {
  width: 100%; resize: vertical;
  border: 1.5px solid #d7d9e0; border-radius: 8px;
  padding: 8px 12px; font: 14px 'Inter', sans-serif;
  margin-bottom: 12px; outline: none;
}
.textarea:focus { border-color: #3b3fe0; box-shadow: 0 0 0 4px rgba(59,63,224,.1); }

.btn-primary {
  background: linear-gradient(135deg, #2b5cff, #5b3bd9);
  color: #fff; border: none; border-radius: 8px;
  padding: 8px 20px; font-size: 14px; font-weight: 500; cursor: pointer;
}
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }

.form-error { color: #d63a51; font-size: 13px; margin: 0 0 8px; }
.form-success { color: #059669; font-size: 13px; margin: 0 0 8px; }

.info-box {
  background: #fef3c7;
  border: 1px solid #fde68a;
  color: #92400e;
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 14px;
  margin: 0;
}

.list { list-style: none; padding: 0; margin: 0; display: grid; gap: 8px; }
.req-item {
  display: flex; gap: 12px; align-items: center; flex-wrap: wrap;
  padding: 8px 12px; background: #f9fafb; border-radius: 8px; font-size: 13px;
}
.badge {
  padding: 2px 8px; border-radius: 6px; font-size: 11px; font-weight: 500;
}
.badge-pending { background: #fef3c7; color: #92400e; }
.badge-approved { background: #d1fae5; color: #065f46; }
.badge-rejected { background: #fee2e2; color: #991b1b; }
.req-date { color: #6b7280; }
.req-reason { color: #4b5563; font-style: italic; }
</style>
