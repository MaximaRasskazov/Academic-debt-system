<script setup>
import { ref, computed, onMounted } from 'vue'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { changeRequestsApi } from '../api/changeRequests'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { usersApi } from '../api/users'

const sidebarOpen = ref(false)

// ── Data ──────────────────────────────────────────────────
const requests  = ref([])
const retakeMap = ref({})
const discMap   = ref({})
const userMap   = ref({})
const loading   = ref(true)
const loadErr   = ref('')

onMounted(async () => {
  try {
    const [reqRes, retakesRes, discsRes, usersRes] = await Promise.allSettled([
      changeRequestsApi.getAll({ limit: 200 }),
      retakesApi.getAll({ limit: 500 }),
      disciplinesApi.getAll({ limit: 500 }),
      usersApi.getAll({ limit: 200 }),
    ])

    if (discsRes.status === 'fulfilled')
      (discsRes.value.data.items ?? discsRes.value.data ?? [])
        .forEach(d => { discMap.value[d.id] = d.name || d.code })

    if (retakesRes.status === 'fulfilled')
      (retakesRes.value.data.items ?? retakesRes.value.data ?? [])
        .forEach(r => { retakeMap.value[r.id] = r })

    if (usersRes.status === 'fulfilled')
      (usersRes.value.data.items ?? usersRes.value.data ?? [])
        .forEach(u => {
          userMap.value[u.id] = [u.last_name, u.first_name, u.middle_name].filter(Boolean).join(' ') || u.email
        })

    if (reqRes.status === 'fulfilled')
      requests.value = reqRes.value.data.items ?? reqRes.value.data ?? []
    else
      loadErr.value = 'Не удалось загрузить заявки'
  } finally {
    loading.value = false
  }
})

// ── Helpers ───────────────────────────────────────────────
function disciplineName(r) {
  const retake = retakeMap.value[r.retake_id]
  return retake ? (discMap.value[retake.discipline_id] || 'Дисциплина') : '—'
}
function retakeDate(r) {
  const retake = retakeMap.value[r.retake_id]
  if (!retake?.scheduled_at) return '—'
  return new Date(retake.scheduled_at).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}
function fmtDate(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}
function fmtTime(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return `${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`
}

function changesSummary(r) {
  const c = r.requested_changes || {}
  const parts = []
  if (c.scheduled_at)     parts.push(`Дата: ${fmtDate(c.scheduled_at)} ${fmtTime(c.scheduled_at)}`)
  if (c.building)         parts.push(`Корпус: ${c.building}`)
  if (c.room)             parts.push(`Ауд.: ${c.room}`)
  if (c.duration_minutes) parts.push(`Длит.: ${c.duration_minutes} мин`)
  if (c.notes)            parts.push(`Заметки: ${c.notes}`)
  return parts.join(' · ') || '—'
}

// ── Approve ───────────────────────────────────────────────
const approving  = ref(null)

async function doApprove(r) {
  approving.value = r.id
  try {
    await changeRequestsApi.approve(r.id)
    requests.value = requests.value.filter(req => req.id !== r.id)
  } catch { /* silent — row stays, user can retry */ }
  finally { approving.value = null }
}

// ── Reject modal ──────────────────────────────────────────
const rejectModal  = ref(null)
const rejectReason = ref('')
const rejecting    = ref(false)
const rejectErr    = ref('')

function openReject(r)  { rejectModal.value = r; rejectReason.value = ''; rejectErr.value = '' }
function closeReject()  { rejectModal.value = null }

async function doReject() {
  if (!rejectReason.value.trim()) { rejectErr.value = 'Укажите причину отклонения'; return }
  rejecting.value = true; rejectErr.value = ''
  try {
    await changeRequestsApi.reject(rejectModal.value.id, rejectReason.value.trim())
    requests.value = requests.value.filter(r => r.id !== rejectModal.value.id)
    closeReject()
  } catch (e) {
    rejectErr.value = e.response?.data?.message || e.response?.data?.error || 'Ошибка'
  } finally { rejecting.value = false }
}
</script>

<template>
  <div class="requests-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-wrap">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">
        <div class="content-wrap">

          <!-- ── Loading ── -->
          <div v-if="loading" class="state-center">
            <div class="spinner" /><span>Загрузка заявок…</span>
          </div>

          <div v-else-if="loadErr" class="state-center state-error">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
            </svg>
            {{ loadErr }}
          </div>

          <template v-else>
            <div class="section-card">

              <div class="table-header">
                <div class="title-row">
                  <h2 class="section-title">Заявки на изменение пересдач</h2>
                  <span class="count-badge">{{ requests.length }}</span>
                </div>
                <p class="subtitle">Новые заявки от преподавателей — ожидают решения</p>
              </div>

              <div v-if="requests.length === 0" class="empty-state">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
                  <path d="M22 12h-6l-2 3H10l-2-3H2"/><path d="M5.45 5.11L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.45-6.89A2 2 0 0016.76 4H7.24a2 2 0 00-1.79 1.11z"/>
                </svg>
                <span>Новых заявок нет</span>
                <p>Когда преподаватели подадут заявки, они появятся здесь</p>
              </div>

              <div v-else class="table-wrap">
                <table class="data-table">
                  <thead>
                    <tr class="head-row">
                      <th>№</th>
                      <th>Дисциплина</th>
                      <th>Преподаватель</th>
                      <th>Запрашиваемые изменения</th>
                      <th>Причина</th>
                      <th>Пересдача</th>
                      <th>Подана</th>
                      <th>Действия</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(r, i) in requests" :key="r.id">
                      <td class="td-num">{{ i + 1 }}</td>
                      <td class="td-subject">{{ disciplineName(r) }}</td>
                      <td class="td-nowrap">{{ userMap[r.requested_by] || '—' }}</td>
                      <td class="td-changes">{{ changesSummary(r) }}</td>
                      <td class="td-reason">{{ r.reason || '—' }}</td>
                      <td class="td-nowrap td-soft">{{ retakeDate(r) }}</td>
                      <td class="td-nowrap td-soft">{{ fmtDate(r.created_at) }}</td>
                      <td>
                        <div class="action-btns">
                          <button
                            class="btn-sm btn-approve-sm"
                            :disabled="approving === r.id"
                            @click="doApprove(r)"
                          >
                            {{ approving === r.id ? '…' : 'Одобрить' }}
                          </button>
                          <button
                            class="btn-sm btn-reject-sm"
                            :disabled="approving === r.id"
                            @click="openReject(r)"
                          >
                            Отклонить
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

            </div>
          </template>

        </div>
      </main>
    </div>

    <!-- ── Reject modal ── -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="rejectModal" class="modal-overlay" @click.self="closeReject">
          <div class="modal">

            <div class="modal-head">
              <span class="modal-title">Отклонить заявку</span>
              <button class="modal-close" @click="closeReject" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                  <path d="M18 6L6 18M6 6l12 12"/>
                </svg>
              </button>
            </div>

            <div class="modal-body">
              <div class="modal-disc">{{ disciplineName(rejectModal) }}</div>
              <div class="modal-teacher">{{ userMap[rejectModal.requested_by] || 'Преподаватель' }}</div>

              <div class="field">
                <label class="field-label">Причина отклонения <span class="required">*</span></label>
                <textarea
                  class="input textarea"
                  v-model="rejectReason"
                  placeholder="Укажите причину — она будет отправлена преподавателю…"
                  rows="4"
                />
              </div>
              <p v-if="rejectErr" class="form-error">{{ rejectErr }}</p>
            </div>

            <div class="modal-foot">
              <button class="btn-outline" @click="closeReject">Отмена</button>
              <button class="btn-reject-confirm" :disabled="rejecting" @click="doReject">
                {{ rejecting ? 'Отклонение…' : 'Отклонить' }}
              </button>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap');

*, *::before, *::after { box-sizing: border-box; }

.requests-root {
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

.page-wrap    { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }
.main         { flex: 1; padding: 24px; }
.content-wrap { max-width: 1200px; display: flex; flex-direction: column; gap: 20px; }

/* ── States ── */
.state-center {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 12px; padding: 80px 20px;
  font: 500 15px/1.5 'Inter', sans-serif; color: var(--ink-soft); text-align: center;
}
.state-error { color: #dc2626; }
.state-error svg { width: 36px; height: 36px; }
.spinner {
  width: 32px; height: 32px; border-radius: 50%;
  border: 3px solid var(--line); border-top-color: var(--brand);
  animation: spin .8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg) } }

/* ── Card ── */
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px;
}

.table-header { margin-bottom: 4px; }
.title-row { display: flex; align-items: center; gap: 10px; margin-bottom: 4px; }
.section-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0;
}
.count-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 24px; height: 22px; padding: 0 7px; border-radius: 11px;
  background: rgba(59,63,224,.1); color: var(--brand-ink);
  font: 600 12px/1 'Inter', sans-serif;
}
.subtitle { font: 13px/1.4 'Inter', sans-serif; color: var(--ink-soft); margin: 0 0 18px; }

/* ── Empty ── */
.empty-state {
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  padding: 48px 20px; color: var(--ink-soft); text-align: center;
}
.empty-state svg { width: 40px; height: 40px; opacity: .3; }
.empty-state span { font: 500 14px/1 'Inter', sans-serif; }
.empty-state p    { font: 13px/1.5 'Inter', sans-serif; color: #9ca3af; margin: 0; }

/* ── Table ── */
.table-wrap { overflow: auto; border: 1px solid var(--line); border-radius: var(--radius); }
.data-table { width: 100%; border-collapse: collapse; font: 13px/1.4 'Inter', sans-serif; }
.head-row th {
  position: sticky; top: 0; z-index: 2;
  background: #f8f9fb; padding: 10px 14px;
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-align: left; white-space: nowrap; border-bottom: 1px solid var(--line);
}
.data-table tbody tr { transition: background .12s; }
.data-table tbody tr:hover { background: rgba(59,63,224,.03); }
.data-table td { padding: 11px 14px; border-bottom: 1px solid var(--line); color: var(--ink); vertical-align: middle; }
.data-table tbody tr:last-child td { border-bottom: none; }
.td-num     { color: var(--ink-soft); width: 36px; }
.td-subject { font-weight: 600; min-width: 140px; }
.td-nowrap  { white-space: nowrap; }
.td-soft    { color: var(--ink-soft); }
.td-changes { max-width: 240px; font-size: 12px; color: var(--brand-ink); }
.td-reason  { max-width: 180px; font-size: 12px; color: var(--ink-soft); }

/* ── Actions ── */
.action-btns { display: flex; gap: 6px; }

.btn-sm { height: 30px; padding: 0 12px; border-radius: 8px; font: 600 12px/1 'Inter', sans-serif; cursor: pointer; white-space: nowrap; border: none; transition: opacity .15s, transform .1s; }
.btn-sm:disabled { opacity: .5; cursor: not-allowed; }

.btn-approve-sm { background: rgba(16,185,129,.12); color: #065f46; }
.btn-approve-sm:hover:not(:disabled) { background: rgba(16,185,129,.2); }

.btn-reject-sm { background: rgba(220,38,38,.08); color: #b91c1c; }
.btn-reject-sm:hover:not(:disabled) { background: rgba(220,38,38,.15); }

/* ── Modal ── */
.modal-overlay {
  position: fixed; inset: 0; z-index: 400;
  background: rgba(10,12,30,.5); backdrop-filter: blur(3px); -webkit-backdrop-filter: blur(3px);
  display: flex; align-items: center; justify-content: center; padding: 20px;
  /* CSS-переменные для teleport-контента */
  --bg:        #f3f4f7;
  --card:      #ffffff;
  --ink:       #1a1d24;
  --ink-soft:  #6b7280;
  --line:      #d7d9e0;
  --brand:     #3b3fe0;
  --brand-ink: #2a2e9e;
  --radius:    10px;
  --ease:      cubic-bezier(.2,.7,.2,1);
  font-family: 'Inter', system-ui, sans-serif;
}
.modal {
  background: var(--card); border-radius: 16px; width: 100%; max-width: 440px;
  box-shadow: 0 24px 64px -12px rgba(10,12,30,.3);
  display: flex; flex-direction: column;
}
.modal-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 20px; border-bottom: 1px solid var(--line);
}
.modal-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6;
}
.modal-close {
  width: 32px; height: 32px; border-radius: 8px;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s;
}
.modal-close:hover { background: var(--bg); }
.modal-close svg { width: 16px; height: 16px; }

.modal-body   { padding: 20px; display: flex; flex-direction: column; gap: 14px; }
.modal-disc   { font: 700 14px/1.3 'Inter', sans-serif; color: var(--ink); }
.modal-teacher{ font: 500 13px/1   'Inter', sans-serif; color: var(--ink-soft); }

.field        { display: flex; flex-direction: column; gap: 6px; }
.field-label  { font: 500 13px/1 'Inter', sans-serif; color: var(--ink); }
.required     { color: #dc2626; }

.input {
  appearance: none; height: 38px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 3px rgba(59,63,224,.1); }
.textarea { height: auto; padding: 10px 12px; resize: vertical; line-height: 1.5; }
.form-error { font: 12px/1.4 'Inter', sans-serif; color: #dc2626; margin: 0; }

.modal-foot {
  display: flex; justify-content: flex-end; gap: 10px;
  padding: 14px 20px; border-top: 1px solid var(--line);
}
.btn-outline {
  padding: 0 18px; height: 40px; background: #fff;
  border: 1.5px solid #c5c7d4; border-radius: var(--radius);
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer;
  transition: border-color .15s, color .15s;
}
.btn-outline:hover { border-color: var(--brand); color: var(--brand); }

.btn-reject-confirm {
  padding: 0 20px; height: 40px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #dc2626, #b91c1c);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 4px 14px -4px rgba(185,28,28,.5);
  transition: transform .15s var(--ease);
}
.btn-reject-confirm:hover:not(:disabled) { transform: scale(1.03); }
.btn-reject-confirm:disabled { opacity: .6; cursor: not-allowed; }

/* ── Modal transition ── */
.modal-enter-active { transition: opacity .2s var(--ease); }
.modal-leave-active { transition: opacity .18s var(--ease); }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .modal { transition: transform .25s var(--ease); }
.modal-leave-active  .modal { transition: transform .2s var(--ease); }
.modal-enter-from .modal, .modal-leave-to .modal { transform: scale(.96) translateY(10px); }

@media (max-width: 640px) { .main { padding: 16px; } }
</style>
