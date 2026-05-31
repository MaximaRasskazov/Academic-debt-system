<script setup>
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import VueDatePicker from '@vuepic/vue-datepicker'
import '@vuepic/vue-datepicker/dist/main.css'
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import { retakesApi } from '../api/retakes'
import { disciplinesApi } from '../api/disciplines'
import { partsInTZ } from '../utils/datetime'

const route = useRoute()
const sidebarOpen = ref(false)

// ── Tabs ──────────────────────────────────────────────────
const activeTab = ref('my-requests')

// ── Shared constants ──────────────────────────────────────
const typeOptions = [
  { value: 'normal',     label: 'Обычная' },
  { value: 'commission', label: 'С комиссией (мин. 3 преподавателя)' },
]
const DURATION_STEP = 5, DURATION_MIN = 15, DURATION_MAX = 480
const STATUS_LABELS = { pending: 'Ожидает', approved: 'Одобрена', rejected: 'Отклонена' }
const TYPE_LABELS   = { normal: 'Обычная', commission: 'С комиссией' }

function clamp(val, min, max) { return Math.max(min, Math.min(max, val || min)) }

// ── Outside click (close dropdowns) ──────────────────────
function handleOutsideClick(e) {
  if (!e.target.closest('.custom-select')) {
    typeDropdownOpen.value = false
    editTypeOpen.value     = false
  }
}
onMounted(async () => {
  document.addEventListener('mousedown', handleOutsideClick)

  // Пре-заполнение при переходе с RetakesPage → кнопка "Изменить"
  const retakeId = route.query.retakeId
  if (retakeId) {
    activeTab.value = 'submit'
    try {
      const [retakeRes, discsRes] = await Promise.allSettled([
        retakesApi.getById(retakeId),
        disciplinesApi.getAll({ limit: 500 }),
      ])
      const discMap = {}
      if (discsRes.status === 'fulfilled')
        (discsRes.value.data.items ?? discsRes.value.data ?? [])
          .forEach(d => { discMap[d.id] = d.name || d.code })

      if (retakeRes.status === 'fulfilled') {
        const r = retakeRes.value.data
        // Предзаполняем форму временем вуза (Ханты, UTC+5), а не зоной устройства.
        const p = r.scheduled_at ? partsInTZ(r.scheduled_at) : null
        form.subject  = discMap[r.discipline_id] || ''
        form.type     = r.kind === 'commission' ? 'commission' : 'normal'
        form.date     = p ? `${p.day}.${p.month}.${p.year}` : null
        form.duration = r.duration_minutes || 90
        form.building = r.building || ''
        form.room     = r.room     || ''
        if (p) {
          hourDisplay.value   = p.hour
          minuteDisplay.value = p.minute
        }
      }
    } catch { /* ignore prefill errors */ }
  }
})
onUnmounted(() => document.removeEventListener('mousedown', handleOutsideClick))

// ── My Requests data ──────────────────────────────────────
const myRequests = ref([])

const statusFilter = ref('all')
const filteredRequests = computed(() =>
  statusFilter.value === 'all'
    ? myRequests.value
    : myRequests.value.filter(r => r.status === statusFilter.value)
)
const pendingCount = computed(() => myRequests.value.filter(r => r.status === 'pending').length)

// ── Submit form ───────────────────────────────────────────
const form = reactive({
  subject: '', type: 'normal', date: null, duration: 90,
  group: '', building: '', room: '', teachers: [], students: [], description: '',
})
const typeDropdownOpen = ref(false)
const hourDisplay      = ref('09')
const minuteDisplay    = ref('00')
const teacherInput     = ref('')
const studentInput     = ref('')
const formError        = ref('')
const formSuccess      = ref(false)

const isCommission   = computed(() => form.type === 'commission')
const minTeachers    = computed(() => isCommission.value ? 3 : 1)
const teacherCountOk = computed(() => form.teachers.length >= minTeachers.value)

function selectType(val) { form.type = val; typeDropdownOpen.value = false }

function onHourBlur()   { const n = clamp(parseInt(hourDisplay.value,   10), 0, 23); hourDisplay.value   = String(n).padStart(2, '0') }
function onMinuteBlur() { const n = clamp(parseInt(minuteDisplay.value, 10), 0, 59); minuteDisplay.value = String(n).padStart(2, '0') }
function onTimeInput(e) { e.target.value = e.target.value.replace(/\D/g, '').slice(0, 2) }

function decreaseDuration() { if (form.duration > DURATION_MIN) form.duration -= DURATION_STEP }
function increaseDuration()  { if (form.duration < DURATION_MAX) form.duration += DURATION_STEP }
function clampDuration()     { form.duration = clamp(form.duration, DURATION_MIN, DURATION_MAX) }

function addTeacher()     { const n = teacherInput.value.trim(); if (!n || (!isCommission.value && form.teachers.length >= 1)) return; form.teachers.push(n); teacherInput.value = '' }
function removeTeacher(i) { form.teachers.splice(i, 1) }
watch(() => form.type, t => { if (t === 'normal' && form.teachers.length > 1) form.teachers.splice(1) })

function addStudent()     { const n = studentInput.value.trim(); if (!n) return; form.students.push(n); studentInput.value = '' }
function removeStudent(i) { form.students.splice(i, 1) }

function resetForm() {
  Object.assign(form, { subject: '', type: 'normal', date: null, duration: 90, group: '', building: '', room: '', teachers: [], students: [], description: '' })
  hourDisplay.value = '09'; minuteDisplay.value = '00'; formError.value = ''
}

function submitForm() {
  formError.value = ''
  if (!form.subject.trim()) { formError.value = 'Укажите дисциплину'; return }
  if (!form.date)           { formError.value = 'Укажите дату'; return }
  if (!teacherCountOk.value) { formError.value = `Минимум ${minTeachers.value} преподавател${minTeachers.value > 1 ? 'я' : 'ь'}`; return }

  const today = new Date().toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
  myRequests.value.unshift({
    id: Date.now(), subject: form.subject, type: form.type, date: form.date,
    time: `${hourDisplay.value}:${minuteDisplay.value}`,
    duration: form.duration, group: form.group, building: form.building, room: form.room,
    teachers: [...form.teachers], students: [...form.students], description: form.description,
    status: 'pending', submittedAt: today, rejectReason: '',
  })
  resetForm()
  formSuccess.value = true
  setTimeout(() => { formSuccess.value = false }, 2500)
  activeTab.value = 'my-requests'
}

// ── Edit modal ────────────────────────────────────────────
const editOpen  = ref(false)
const editId    = ref(null)
const editForm  = reactive({
  subject: '', type: 'normal', date: null, duration: 90,
  group: '', building: '', room: '', teachers: [], students: [], description: '',
})
const editTypeOpen     = ref(false)
const editHour         = ref('09')
const editMinute       = ref('00')
const editTeacherInput = ref('')
const editStudentInput = ref('')
const editFormError    = ref('')

const editIsCommission   = computed(() => editForm.type === 'commission')
const editMinTeachers    = computed(() => editIsCommission.value ? 3 : 1)
const editTeacherCountOk = computed(() => editForm.teachers.length >= editMinTeachers.value)

function selectEditType(val) { editForm.type = val; editTypeOpen.value = false }

function onEditHourBlur()   { const n = clamp(parseInt(editHour.value,   10), 0, 23); editHour.value   = String(n).padStart(2, '0') }
function onEditMinuteBlur() { const n = clamp(parseInt(editMinute.value, 10), 0, 59); editMinute.value = String(n).padStart(2, '0') }

function editDecreaseDuration() { if (editForm.duration > DURATION_MIN) editForm.duration -= DURATION_STEP }
function editIncreaseDuration()  { if (editForm.duration < DURATION_MAX) editForm.duration += DURATION_STEP }
function editClampDuration()     { editForm.duration = clamp(editForm.duration, DURATION_MIN, DURATION_MAX) }

function addEditTeacher()     { const n = editTeacherInput.value.trim(); if (!n || (!editIsCommission.value && editForm.teachers.length >= 1)) return; editForm.teachers.push(n); editTeacherInput.value = '' }
function removeEditTeacher(i) { editForm.teachers.splice(i, 1) }
watch(() => editForm.type, t => { if (t === 'normal' && editForm.teachers.length > 1) editForm.teachers.splice(1) })

function addEditStudent()     { const n = editStudentInput.value.trim(); if (!n) return; editForm.students.push(n); editStudentInput.value = '' }
function removeEditStudent(i) { editForm.students.splice(i, 1) }

function openEditModal(r) {
  editId.value = r.id
  Object.assign(editForm, { subject: r.subject, type: r.type, date: r.date, duration: r.duration, group: r.group, building: r.building, room: r.room, teachers: [...r.teachers], students: [...r.students], description: r.description })
  const [h, m] = r.time.split(':'); editHour.value = h; editMinute.value = m
  editFormError.value = ''; editOpen.value = true
}
function closeEditModal() { editOpen.value = false }

function saveEdit() {
  editFormError.value = ''
  if (!editForm.subject.trim()) { editFormError.value = 'Укажите дисциплину'; return }
  if (!editForm.date)           { editFormError.value = 'Укажите дату'; return }
  if (!editTeacherCountOk.value) { editFormError.value = `Минимум ${editMinTeachers.value} преподавател${editMinTeachers.value > 1 ? 'я' : 'ь'}`; return }

  const idx = myRequests.value.findIndex(r => r.id === editId.value)
  if (idx !== -1) {
    Object.assign(myRequests.value[idx], {
      subject: editForm.subject, type: editForm.type, date: editForm.date,
      time: `${editHour.value}:${editMinute.value}`,
      duration: editForm.duration, group: editForm.group, building: editForm.building,
      room: editForm.room, teachers: [...editForm.teachers], students: [...editForm.students],
      description: editForm.description, status: 'pending',
    })
  }
  closeEditModal()
}

// ── Detail modal ──────────────────────────────────────────
const detailModal = ref(null)
function openDetail(r)  { detailModal.value = r }
function closeDetail()  { detailModal.value = null }
</script>

<template>
  <div class="teacher-req-root">

    <AppSidebar :open="sidebarOpen" @close="sidebarOpen = false" />

    <div class="page-wrap">
      <AppHeader @open-sidebar="sidebarOpen = true" />

      <main class="main">
        <div class="content-wrap">

          <!-- ── Tab switcher ── -->
          <div class="page-tabs">
            <button class="tab-btn" :class="{ active: activeTab === 'my-requests' }" @click="activeTab = 'my-requests'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 12h-6l-2 3H10l-2-3H2"/><path d="M5.45 5.11L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.45-6.89A2 2 0 0016.76 4H7.24a2 2 0 00-1.79 1.11z"/>
              </svg>
              Мои заявки
              <span v-if="pendingCount > 0" class="tab-badge">{{ pendingCount }}</span>
            </button>
            <button class="tab-btn" :class="{ active: activeTab === 'submit' }" @click="activeTab = 'submit'">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="16"/><line x1="8" y1="12" x2="16" y2="12"/>
              </svg>
              Подать заявку
            </button>
          </div>

          <!-- ── My Requests tab ── -->
          <div v-if="activeTab === 'my-requests'" class="section-card">
            <div class="table-header">
              <h2 class="section-title" style="margin:0">Мои заявки</h2>
              <div class="table-header-actions">
                <select class="filter-select" v-model="statusFilter">
                  <option value="all">Все статусы</option>
                  <option value="pending">Ожидает</option>
                  <option value="approved">Одобрена</option>
                  <option value="rejected">Отклонена</option>
                </select>
                <button class="btn-primary btn-sm-primary" @click="activeTab = 'submit'">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                    <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
                  </svg>
                  Подать заявку
                </button>
              </div>
            </div>

            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr class="head-row">
                    <th>№</th>
                    <th>Дисциплина</th>
                    <th>Тип</th>
                    <th>Дата</th>
                    <th>Группа</th>
                    <th>Статус</th>
                    <th>Подана</th>
                    <th>Действия</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, i) in filteredRequests" :key="r.id">
                    <td class="td-num">{{ i + 1 }}</td>
                    <td class="td-subject">{{ r.subject }}</td>
                    <td>{{ TYPE_LABELS[r.type] }}</td>
                    <td class="td-nowrap">{{ r.date }}</td>
                    <td>{{ r.group || '—' }}</td>
                    <td>
                      <span class="status-badge" :class="'status-' + r.status">
                        {{ STATUS_LABELS[r.status] }}
                      </span>
                    </td>
                    <td class="td-nowrap td-soft">{{ r.submittedAt }}</td>
                    <td>
                      <div class="action-btns">
                        <button class="btn-sm btn-outline" @click="openDetail(r)">Подробнее</button>
                        <button class="btn-sm btn-primary" @click="openEditModal(r)">Изменить</button>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="filteredRequests.length === 0">
                    <td colspan="8" class="empty-row">
                      <div class="empty-cell">
                        <span>Заявок пока нет</span>
                        <button class="btn-link" @click="activeTab = 'submit'">Подать первую заявку →</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- ── Submit tab ── -->
          <div v-if="activeTab === 'submit'" class="section-card">
            <h2 class="section-title">Подать заявку на пересдачу</h2>

            <form class="retake-form" @submit.prevent="submitForm" novalidate>

              <div class="form-row">
                <div class="field" style="flex:2">
                  <label>Дисциплина</label>
                  <input class="input" v-model="form.subject" placeholder="Название дисциплины" />
                </div>
                <div class="field">
                  <label>Тип пересдачи</label>
                  <div class="custom-select" :class="{ open: typeDropdownOpen }">
                    <button type="button" class="custom-select-trigger" @click="typeDropdownOpen = !typeDropdownOpen">
                      <span>{{ typeOptions.find(o => o.value === form.type)?.label }}</span>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 9l6 6 6-6"/></svg>
                    </button>
                    <div class="custom-select-dropdown">
                      <button v-for="opt in typeOptions" :key="opt.value" type="button" class="custom-select-option" :class="{ selected: form.type === opt.value }" @click="selectType(opt.value)">
                        {{ opt.label }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              <div class="form-row">
                <div class="field" style="flex:2">
                  <label>Дата</label>
                  <VueDatePicker v-model="form.date" locale="ru" format="dd.MM.yyyy" model-type="format" :enable-time-picker="false" auto-apply placeholder="дд.мм.гггг" />
                </div>
                <div class="field field--shrink">
                  <label>Время</label>
                  <div class="time-picker">
                    <input class="time-input" type="text" inputmode="numeric" v-model="hourDisplay"   maxlength="2" placeholder="09" @input="onTimeInput" @blur="onHourBlur" />
                    <span class="time-colon">:</span>
                    <input class="time-input" type="text" inputmode="numeric" v-model="minuteDisplay" maxlength="2" placeholder="00" @input="onTimeInput" @blur="onMinuteBlur" />
                  </div>
                </div>
                <div class="field">
                  <label>Длительность (мин)</label>
                  <div class="stepper">
                    <button type="button" class="stepper-btn" @click="decreaseDuration" :disabled="form.duration <= DURATION_MIN">−</button>
                    <input class="stepper-input" type="number" v-model.number="form.duration" min="15" max="480" @blur="clampDuration" />
                    <button type="button" class="stepper-btn" @click="increaseDuration"  :disabled="form.duration >= DURATION_MAX">+</button>
                  </div>
                </div>
              </div>

              <div class="form-row">
                <div class="field field--full">
                  <label>{{ isCommission ? 'Преподаватели комиссии' : 'Преподаватель' }}</label>
                  <div class="tags-wrap">
                    <span v-for="(t, i) in form.teachers" :key="'t'+i" class="tag">{{ t }}<button type="button" class="tag-remove" @click="removeTeacher(i)">×</button></span>
                    <input v-if="isCommission || form.teachers.length === 0" class="tag-input" v-model="teacherInput"
                           :placeholder="isCommission ? 'ФИО преподавателя, затем Enter' : 'ФИО преподавателя'"
                           @keydown.enter.prevent="addTeacher" />
                  </div>
                  <p v-if="isCommission && form.teachers.length > 0 && !teacherCountOk" class="hint-warn">Для пересдачи с комиссией необходимо минимум 3 преподавателя</p>
                </div>
              </div>

              <div class="form-row">
                <div class="field field--full">
                  <label>Студенты</label>
                  <div class="tags-wrap">
                    <span v-for="(s, i) in form.students" :key="'s'+i" class="tag tag--student">{{ s }}<button type="button" class="tag-remove" @click="removeStudent(i)">×</button></span>
                    <input class="tag-input" v-model="studentInput" placeholder="ФИО студента, затем Enter" @keydown.enter.prevent="addStudent" />
                  </div>
                </div>
              </div>

              <div class="form-row">
                <div class="field">
                  <label>Группа</label>
                  <input class="input" v-model="form.group" placeholder="Название группы" />
                </div>
                <div class="field">
                  <label>Корпус</label>
                  <input class="input" v-model="form.building" placeholder="№ корпуса" />
                </div>
                <div class="field">
                  <label>Аудитория</label>
                  <input class="input" v-model="form.room" placeholder="№ аудитории" />
                </div>
              </div>

              <div class="form-row">
                <div class="field field--full">
                  <label>Описание / причина</label>
                  <textarea class="input textarea" v-model="form.description" placeholder="Причина или дополнительные сведения…" rows="3" />
                </div>
              </div>

              <p v-if="formError"   class="form-msg form-error">{{ formError }}</p>
              <p v-if="formSuccess" class="form-msg form-success">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M20 6L9 17l-5-5"/></svg>
                Заявка успешно подана
              </p>

              <button class="btn-submit" type="submit">Отправить заявку</button>

            </form>
          </div>

        </div>
      </main>
    </div>

    <!-- ── Detail modal ── -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="detailModal" class="modal-overlay" @click.self="closeDetail">
          <div class="modal modal--detail">

            <div class="modal-head">
              <span class="modal-title">{{ detailModal.subject }}</span>
              <button class="modal-close" @click="closeDetail" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
              </button>
            </div>

            <div class="modal-body">

              <!-- Status hero -->
              <div class="detail-hero" :class="'hero-' + detailModal.status">
                <div class="detail-hero-icon">
                  <svg v-if="detailModal.status === 'pending'"  viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  <svg v-if="detailModal.status === 'approved'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
                  <svg v-if="detailModal.status === 'rejected'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
                </div>
                <div>
                  <div class="detail-hero-status">{{ STATUS_LABELS[detailModal.status] }}</div>
                  <div class="detail-hero-date">Подана {{ detailModal.submittedAt }}</div>
                </div>
              </div>

              <!-- Reject reason -->
              <div v-if="detailModal.status === 'rejected' && detailModal.rejectReason" class="reject-banner">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
                <div><strong>Причина отказа:</strong> {{ detailModal.rejectReason }}</div>
              </div>

              <!-- Info grid -->
              <div class="detail-info-grid">
                <div class="detail-info-cell">
                  <span class="dil">Тип</span>
                  <span class="div">{{ TYPE_LABELS[detailModal.type] }}</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Дата и время</span>
                  <span class="div">{{ detailModal.date }} в {{ detailModal.time }}</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Длительность</span>
                  <span class="div">{{ detailModal.duration }} мин</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Группа</span>
                  <span class="div">{{ detailModal.group || '—' }}</span>
                </div>
                <div class="detail-info-cell">
                  <span class="dil">Корпус / Аудитория</span>
                  <span class="div">{{ detailModal.building || '—' }} / {{ detailModal.room || '—' }}</span>
                </div>
              </div>

              <!-- People -->
              <div class="detail-people">
                <div class="detail-people-col">
                  <span class="dil">{{ detailModal.type === 'commission' ? 'Преподаватели' : 'Преподаватель' }}</span>
                  <div class="detail-tags">
                    <span v-for="t in detailModal.teachers" :key="t" class="tag">{{ t }}</span>
                    <span v-if="!detailModal.teachers.length" class="div">—</span>
                  </div>
                </div>
                <div class="detail-people-col">
                  <span class="dil">Студенты</span>
                  <div class="detail-tags">
                    <span v-for="s in detailModal.students" :key="s" class="tag tag--student">{{ s }}</span>
                    <span v-if="!detailModal.students.length" class="div">—</span>
                  </div>
                </div>
              </div>

              <!-- Description -->
              <div v-if="detailModal.description" class="detail-description">
                <span class="dil">Описание</span>
                <p class="div">{{ detailModal.description }}</p>
              </div>

            </div>

            <div class="modal-foot">
              <button class="btn-outline" @click="closeDetail">Закрыть</button>
              <button class="btn-primary" @click="() => { const r = detailModal; closeDetail(); openEditModal(r) }">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                Изменить заявку
              </button>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ── Edit modal ── -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="editOpen" class="modal-overlay" @click.self="closeEditModal">
          <div class="modal modal--edit">

            <div class="modal-head">
              <span class="modal-title">Редактировать заявку</span>
              <button class="modal-close" @click="closeEditModal" aria-label="Закрыть">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
              </button>
            </div>

            <div class="modal-body modal-body--form">
              <form class="retake-form" @submit.prevent="saveEdit" novalidate>

                <div class="form-row">
                  <div class="field" style="flex:2">
                    <label>Дисциплина</label>
                    <input class="input" v-model="editForm.subject" placeholder="Название дисциплины" />
                  </div>
                  <div class="field">
                    <label>Тип пересдачи</label>
                    <div class="custom-select" :class="{ open: editTypeOpen }">
                      <button type="button" class="custom-select-trigger" @click="editTypeOpen = !editTypeOpen">
                        <span>{{ typeOptions.find(o => o.value === editForm.type)?.label }}</span>
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 9l6 6 6-6"/></svg>
                      </button>
                      <div class="custom-select-dropdown">
                        <button v-for="opt in typeOptions" :key="opt.value" type="button" class="custom-select-option" :class="{ selected: editForm.type === opt.value }" @click="selectEditType(opt.value)">
                          {{ opt.label }}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="form-row">
                  <div class="field" style="flex:2">
                    <label>Дата</label>
                    <VueDatePicker v-model="editForm.date" locale="ru" format="dd.MM.yyyy" model-type="format" :enable-time-picker="false" auto-apply placeholder="дд.мм.гггг" />
                  </div>
                  <div class="field field--shrink">
                    <label>Время</label>
                    <div class="time-picker">
                      <input class="time-input" type="text" inputmode="numeric" v-model="editHour"   maxlength="2" placeholder="00" @input="onTimeInput" @blur="onEditHourBlur" />
                      <span class="time-colon">:</span>
                      <input class="time-input" type="text" inputmode="numeric" v-model="editMinute" maxlength="2" placeholder="00" @input="onTimeInput" @blur="onEditMinuteBlur" />
                    </div>
                  </div>
                  <div class="field">
                    <label>Длительность (мин)</label>
                    <div class="stepper">
                      <button type="button" class="stepper-btn" @click="editDecreaseDuration" :disabled="editForm.duration <= DURATION_MIN">−</button>
                      <input class="stepper-input" type="number" v-model.number="editForm.duration" min="15" max="480" @blur="editClampDuration" />
                      <button type="button" class="stepper-btn" @click="editIncreaseDuration"  :disabled="editForm.duration >= DURATION_MAX">+</button>
                    </div>
                  </div>
                </div>

                <div class="form-row">
                  <div class="field field--full">
                    <label>{{ editIsCommission ? 'Преподаватели' : 'Преподаватель' }}</label>
                    <div class="tags-wrap">
                      <span v-for="(t, i) in editForm.teachers" :key="'et'+i" class="tag">{{ t }}<button type="button" class="tag-remove" @click="removeEditTeacher(i)">×</button></span>
                      <input v-if="editIsCommission || editForm.teachers.length === 0" class="tag-input" v-model="editTeacherInput" :placeholder="editIsCommission ? 'ФИО преподавателя, затем Enter' : 'ФИО преподавателя'" @keydown.enter.prevent="addEditTeacher" />
                    </div>
                    <p v-if="editIsCommission && editForm.teachers.length > 0 && !editTeacherCountOk" class="hint-warn">Минимум 3 преподавателя для комиссии</p>
                  </div>
                </div>

                <div class="form-row">
                  <div class="field field--full">
                    <label>Студенты</label>
                    <div class="tags-wrap">
                      <span v-for="(s, i) in editForm.students" :key="'es'+i" class="tag tag--student">{{ s }}<button type="button" class="tag-remove" @click="removeEditStudent(i)">×</button></span>
                      <input class="tag-input" v-model="editStudentInput" placeholder="ФИО студента, затем Enter" @keydown.enter.prevent="addEditStudent" />
                    </div>
                  </div>
                </div>

                <div class="form-row">
                  <div class="field">
                    <label>Группа</label>
                    <input class="input" v-model="editForm.group" placeholder="Название группы" />
                  </div>
                  <div class="field">
                    <label>Корпус</label>
                    <input class="input" v-model="editForm.building" placeholder="№ корпуса" />
                  </div>
                  <div class="field">
                    <label>Аудитория</label>
                    <input class="input" v-model="editForm.room" placeholder="№ аудитории" />
                  </div>
                </div>

                <div class="form-row">
                  <div class="field field--full">
                    <label>Описание</label>
                    <textarea class="input textarea" v-model="editForm.description" placeholder="Причина или дополнительные сведения..." rows="3" />
                  </div>
                </div>

                <p v-if="editFormError" class="form-msg form-error">{{ editFormError }}</p>

              </form>
            </div>

            <div class="modal-foot">
              <button class="btn-outline" type="button" @click="closeEditModal">Отмена</button>
              <button class="btn-primary" type="button" @click="saveEdit">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M20 6L9 17l-5-5"/></svg>
                Сохранить изменения
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

.teacher-req-root {
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

.page-wrap { min-height: 100dvh; background: var(--bg); display: flex; flex-direction: column; }

.main { flex: 1; padding: 24px; display: flex; justify-content: center; }

.content-wrap { width: 100%; display: flex; flex-direction: column; gap: 20px; }

/* ── Tab switcher ── */
.page-tabs {
  display: flex; gap: 6px;
  background: var(--card); border-radius: 14px; padding: 6px;
  box-shadow: var(--shadow);
}
.tab-btn {
  flex: 1; height: 44px; border: none; border-radius: 10px;
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink-soft); background: transparent;
  cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px;
  transition: background .2s var(--ease), color .2s var(--ease), box-shadow .2s var(--ease);
  position: relative;
}
.tab-btn svg { width: 16px; height: 16px; flex-shrink: 0; }
.tab-btn.active {
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff;
  box-shadow: 0 4px 14px -4px rgba(91,59,217,.5);
}
.tab-btn:not(.active):hover { background: rgba(59,63,224,.06); color: var(--brand); }
.tab-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 20px; padding: 0 5px; border-radius: 10px;
  background: rgba(245,158,11,.25); color: #b45309;
  font: 700 11px/1 'Inter', sans-serif;
}
.tab-btn.active .tab-badge { background: rgba(255,255,255,.25); color: #fff; }

/* ── Section card ── */
.section-card {
  background: var(--card); border-radius: var(--radius);
  box-shadow: var(--shadow); padding: 24px;
}
.section-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 600; color: #3C38B6; margin: 0 0 20px;
}

/* ── Table header ── */
.table-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 16px; gap: 12px; flex-wrap: wrap;
}
.filter-select {
  appearance: none; height: 36px; padding: 0 32px 0 12px;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%236b7280' stroke-width='2' stroke-linecap='round'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E") no-repeat right 8px center;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer; outline: none;
  transition: border-color .2s;
}
.filter-select:focus { border-color: var(--brand); }

/* ── Table ── */
.table-wrap {
  overflow: auto; border: 1px solid var(--line); border-radius: var(--radius);
}
.data-table { width: 100%; border-collapse: collapse; font: 13px/1.4 'Inter', sans-serif; }
.head-row th {
  position: sticky; top: 0; z-index: 2;
  background: #f8f9fb; padding: 10px 14px;
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-align: left; white-space: nowrap; border-bottom: 1px solid var(--line);
}
.data-table tbody tr { transition: background .12s; }
.data-table tbody tr:hover { background: rgba(59,63,224,.03); }
.data-table td { padding: 11px 14px; border-bottom: 1px solid var(--line); color: var(--ink); vertical-align: middle; text-align: left; }
.data-table tbody tr:last-child td { border-bottom: none; }
.td-num     { color: var(--ink-soft); width: 40px; }
.td-subject { font-weight: 500; }
.td-nowrap  { white-space: nowrap; }
.td-soft    { color: var(--ink-soft); }
.empty-row  { text-align: center; color: var(--ink-soft); padding: 36px !important; }
.empty-cell { display: flex; flex-direction: column; align-items: center; gap: 8px; }
.btn-link   { background: none; border: none; cursor: pointer; color: var(--brand); font: 500 13px/1 'Inter', sans-serif; text-decoration: underline; padding: 0; }

.table-header-actions { display: flex; align-items: center; gap: 10px; }
.btn-sm-primary {
  display: inline-flex; align-items: center; gap: 6px;
  height: 36px !important; padding: 0 16px !important; font-size: 13px !important;
}
.btn-sm-primary svg { width: 14px; height: 14px; }

/* ── Action buttons ── */
.action-btns { display: flex; gap: 6px; }
.btn-sm { height: 30px !important; padding: 0 12px !important; font-size: 12px !important; }

/* ── Status badges ── */
.status-badge {
  display: inline-block; padding: 3px 10px; border-radius: 20px;
  font: 600 11px/1.4 'Inter', sans-serif; white-space: nowrap;
}
.status-pending  { background: rgba(245,158,11,.12); color: #b45309; }
.status-approved { background: rgba(16,185,129,.12);  color: #065f46; }
.status-rejected { background: rgba(220,38,38,.10);   color: #b91c1c; }

/* ── Form ── */
.retake-form { display: flex; flex-direction: column; gap: 16px; }
.form-row { display: flex; gap: 16px; }
.field { flex: 1; display: flex; flex-direction: column; gap: 6px; }
.field--shrink { flex: none; }
.field--full   { width: 100%; }
.field label   { font-size: 13px; font-weight: 500; color: var(--ink); text-align: left; }

.input {
  appearance: none; height: 38px; width: 100%;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.input::placeholder { color: #b7b9c2; }
.textarea { height: auto; padding: 10px 12px; resize: vertical; line-height: 1.5; }

/* ── Time picker ── */
.time-picker { display: flex; align-items: center; gap: 6px; }
.time-input {
  height: 38px; width: 64px; text-align: center;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 8px;
  font: 14px/1 'Inter', sans-serif; color: var(--ink); outline: none;
  transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
  -moz-appearance: textfield;
}
.time-input::-webkit-outer-spin-button,
.time-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }
.time-input:focus { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.time-colon { font-size: 18px; font-weight: 700; color: var(--ink-soft); user-select: none; }

/* ── Duration stepper ── */
.stepper { display: flex; align-items: stretch; height: 38px; border: 1.5px solid var(--line); border-radius: var(--radius); overflow: hidden; background: #fff; }
.stepper-btn { width: 38px; flex-shrink: 0; background: none; border: none; font-size: 20px; color: var(--ink-soft); cursor: pointer; display: grid; place-items: center; transition: background .15s, color .15s; }
.stepper-btn:hover:not(:disabled) { background: rgba(59,63,224,.07); color: var(--brand); }
.stepper-btn:disabled { opacity: .35; cursor: not-allowed; }
.stepper-input { flex: 1; border: none; border-left: 1px solid var(--line); border-right: 1px solid var(--line); background: transparent; outline: none; font: 500 13px/1 'Inter', sans-serif; color: var(--ink); text-align: center; -moz-appearance: textfield; }
.stepper-input::-webkit-outer-spin-button,
.stepper-input::-webkit-inner-spin-button { -webkit-appearance: none; margin: 0; }

/* ── Tags ── */
.tags-wrap {
  min-height: 40px; border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 4px 8px;
  display: flex; flex-wrap: wrap; gap: 6px; align-items: center;
  cursor: text; transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.tags-wrap:focus-within { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); }
.tag {
  display: inline-flex; align-items: center; gap: 4px; padding: 4px 6px 4px 10px;
  background: rgba(59,63,224,.1); color: var(--brand-ink); border-radius: 20px; font-size: 12px; font-weight: 500;
}
.tag--student { background: rgba(16,185,129,.1); color: #065f46; }
.tag-remove { background: none; border: none; cursor: pointer; color: inherit; font-size: 15px; line-height: 1; padding: 0 2px; opacity: .6; transition: opacity .15s; }
.tag-remove:hover { opacity: 1; }
.tag-input { border: none; outline: none; flex: 1; min-width: 180px; font: 13px/1 'Inter', sans-serif; color: var(--ink); background: transparent; padding: 4px 0; }
.tag-input::placeholder { color: #b7b9c2; }
.hint-warn { font-size: 12px; color: #d97706; margin: 4px 0 0; }

/* ── Custom select ── */
.custom-select { position: relative; }
.custom-select-trigger {
  appearance: none; width: 100%; height: 38px; text-align: left;
  border: 1.5px solid var(--line); border-radius: var(--radius);
  background: #fff; padding: 0 36px 0 12px;
  font: 13px/1 'Inter', sans-serif; color: var(--ink);
  display: flex; align-items: center; justify-content: space-between;
  cursor: pointer; transition: border-color .2s var(--ease), box-shadow .2s var(--ease);
}
.custom-select-trigger:focus,
.custom-select.open .custom-select-trigger { border-color: var(--brand); box-shadow: 0 0 0 4px rgba(59,63,224,.12); outline: none; }
.custom-select-trigger svg { width: 16px; height: 16px; flex-shrink: 0; color: var(--ink-soft); transition: transform .2s var(--ease); position: absolute; right: 10px; }
.custom-select.open .custom-select-trigger svg { transform: rotate(180deg); }
.custom-select-dropdown {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: #fff; border: 1.5px solid var(--line); border-radius: var(--radius);
  box-shadow: 0 8px 24px -4px rgba(20,22,60,.12); z-index: 50; overflow: hidden;
  opacity: 0; pointer-events: none; transform: translateY(-4px);
  transition: opacity .15s var(--ease), transform .15s var(--ease);
}
.custom-select.open .custom-select-dropdown { opacity: 1; pointer-events: all; transform: translateY(0); }
.custom-select-option { display: block; width: 100%; text-align: left; background: none; border: none; padding: 10px 14px; font: 13px/1.4 'Inter', sans-serif; color: var(--ink); cursor: pointer; transition: background .12s; }
.custom-select-option:hover { background: rgba(59,63,224,.06); }
.custom-select-option.selected { color: var(--brand); font-weight: 500; background: rgba(59,63,224,.05); }

/* ── Form messages & submit ── */
.form-msg { font: 13px/1.4 'Inter', sans-serif; margin: 0; display: flex; align-items: center; gap: 6px; }
.form-error   { color: #dc2626; }
.form-success { color: #059669; }

/* ── Real API submit form extras ── */
.loading-hint { font: 13px/1.5 'Inter', sans-serif; color: var(--ink-soft); padding: 10px 0; }

.current-values {
  background: var(--bg); border-radius: 10px; padding: 14px 16px;
  display: flex; flex-direction: column; gap: 8px;
}
.cv-label { font: 600 11px/1 'Inter', sans-serif; color: var(--ink-soft); text-transform: uppercase; letter-spacing: .05em; }
.cv-grid  { display: flex; flex-wrap: wrap; gap: 12px 24px; }
.cv-item  { display: flex; flex-direction: column; gap: 3px; }
.cv-key   { font: 500 11px/1 'Inter', sans-serif; color: var(--ink-soft); }
.cv-val   { font: 600 13px/1 'Inter', sans-serif; color: var(--ink); }

.changes-header {
  font: 600 12px/1 'Inter', sans-serif; color: var(--ink-soft);
  text-transform: uppercase; letter-spacing: .05em;
  padding-bottom: 4px; border-bottom: 1px solid var(--line);
}
.form-success svg { width: 15px; height: 15px; }

.btn-submit {
  align-self: flex-start; padding: 0 32px; height: 42px; border: none;
  border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 14px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 8px 20px -8px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-submit:hover { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }

/* ── Buttons ── */
.btn-primary {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 0 18px; height: 40px; border: none; border-radius: var(--radius);
  background: linear-gradient(135deg, #2b5cff 0%, #5b3bd9 55%, #8b3df0 100%);
  color: #fff; font: 600 13px/1 'Inter', sans-serif; cursor: pointer;
  box-shadow: 0 6px 18px -6px rgba(91,59,217,.55);
  transition: transform .2s var(--ease), box-shadow .2s var(--ease);
}
.btn-primary:hover { transform: scale(1.03); box-shadow: 0 4px 10px -5px rgba(91,59,217,.7); }
.btn-primary svg { width: 15px; height: 15px; }

.btn-outline {
  padding: 0 18px; height: 40px;
  background: #fff; border: 1.5px solid #c5c7d4; border-radius: var(--radius);
  font: 600 13px/1 'Inter', sans-serif; color: var(--ink); cursor: pointer;
  box-shadow: 0 1px 4px rgba(20,22,60,.08);
  transition: border-color .15s, color .15s, box-shadow .15s;
}
.btn-outline:hover { border-color: var(--brand); color: var(--brand); box-shadow: 0 2px 10px rgba(59,63,224,.14); }

/* ── Modal overlay ── */
.modal-overlay {
  position: fixed; inset: 0; z-index: 400;
  background: rgba(10,12,30,.5);
  backdrop-filter: blur(3px); -webkit-backdrop-filter: blur(3px);
  display: flex; align-items: center; justify-content: center;
  padding: 20px;
  /* CSS-переменные для teleport-контента (выходит за пределы .teacher-req-root) */
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
  font-family: 'Inter', system-ui, sans-serif;
}

/* ── Modal base ── */
.modal {
  background: var(--card); border-radius: 16px;
  box-shadow: 0 24px 64px -12px rgba(10,12,30,.3);
  width: 100%; display: flex; flex-direction: column; overflow: hidden;
  max-height: 90vh;
}
.modal--detail { max-width: 520px; }
.modal--edit   { max-width: 720px; }

.modal-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 18px 20px; border-bottom: 1px solid var(--line); flex-shrink: 0;
}
.modal-title {
  font-family: 'Gerhaus', 'Regular', 'Inter', sans-serif;
  font-size: 15px; font-weight: 700; color: #3C38B6;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: calc(100% - 48px);
}
.modal-close {
  width: 32px; height: 32px; border-radius: 8px; flex-shrink: 0;
  background: none; border: none; cursor: pointer;
  display: grid; place-items: center; color: var(--ink-soft);
  transition: background .15s, color .15s;
}
.modal-close:hover { background: var(--bg); color: var(--ink); }
.modal-close svg { width: 16px; height: 16px; }

.modal-body { flex: 1; overflow-y: auto; padding: 20px; }
.modal-body::-webkit-scrollbar { width: 4px; }
.modal-body::-webkit-scrollbar-thumb { background: var(--line); border-radius: 4px; }
.modal-body--form { padding: 20px 20px 4px; }

.modal-foot {
  display: flex; justify-content: flex-end; gap: 10px;
  padding: 14px 20px; border-top: 1px solid var(--line); flex-shrink: 0;
}

/* ── Detail modal content ── */
.detail-hero {
  display: flex; align-items: center; gap: 14px;
  padding: 14px 16px; border-radius: 12px; margin-bottom: 18px;
}
.hero-pending  { background: rgba(245,158,11,.08); border: 1px solid rgba(245,158,11,.2); }
.hero-approved { background: rgba(16,185,129,.08); border: 1px solid rgba(16,185,129,.2); }
.hero-rejected { background: rgba(220,38,38,.06);  border: 1px solid rgba(220,38,38,.15); }

.detail-hero-icon {
  width: 42px; height: 42px; border-radius: 50%;
  display: grid; place-items: center; flex-shrink: 0;
}
.hero-pending  .detail-hero-icon { background: rgba(245,158,11,.15); color: #b45309; }
.hero-approved .detail-hero-icon { background: rgba(16,185,129,.18);  color: #065f46; }
.hero-rejected .detail-hero-icon { background: rgba(220,38,38,.12);   color: #b91c1c; }
.detail-hero-icon svg { width: 20px; height: 20px; }

.detail-hero-status { font: 700 14px/1 'Inter', sans-serif; color: var(--ink); margin-bottom: 4px; }
.detail-hero-date   { font: 12px/1 'Inter', sans-serif; color: var(--ink-soft); }

.reject-banner {
  display: flex; align-items: flex-start; gap: 10px;
  background: rgba(220,38,38,.06); border: 1px solid rgba(220,38,38,.2);
  border-radius: 10px; padding: 12px 14px;
  font: 13px/1.5 'Inter', sans-serif; color: #b91c1c; margin-bottom: 16px;
}
.reject-banner svg { width: 16px; height: 16px; flex-shrink: 0; margin-top: 1px; }

.detail-info-grid {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 0; border: 1px solid var(--line); border-radius: 10px; overflow: hidden;
  margin-bottom: 16px;
}
.detail-info-cell {
  display: flex; flex-direction: column; gap: 4px;
  padding: 12px 14px; border-bottom: 1px solid var(--line);
  border-right: 1px solid var(--line);
}
.detail-info-cell:nth-child(2n) { border-right: none; }
.detail-info-cell:nth-last-child(-n+2) { border-bottom: none; }

.detail-people {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 12px; margin-bottom: 16px;
}
.detail-people-col { display: flex; flex-direction: column; gap: 8px; }
.detail-tags { display: flex; flex-wrap: wrap; gap: 6px; }

.detail-description {
  display: flex; flex-direction: column; gap: 6px;
  padding: 12px 14px; background: var(--bg); border-radius: 10px;
}
.detail-description .div { margin: 0; white-space: pre-wrap; }

.dil {
  font: 600 11px/1 'Inter', sans-serif;
  color: var(--ink-soft); text-transform: uppercase; letter-spacing: .05em;
}
.div { font: 13px/1.5 'Inter', sans-serif; color: var(--ink); }

/* ── Transition ── */
.modal-enter-active { transition: opacity .2s var(--ease); }
.modal-leave-active { transition: opacity .18s var(--ease); }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-active .modal { transition: transform .25s var(--ease); }
.modal-leave-active  .modal { transition: transform .2s  var(--ease); }
.modal-enter-from .modal, .modal-leave-to .modal { transform: scale(.96) translateY(12px); }

/* ── Responsive ── */
@media (max-width: 640px) {
  .main { padding: 16px; }
  .form-row { flex-direction: column; gap: 12px; }
  .detail-info-grid { grid-template-columns: 1fr; }
  .detail-info-cell { border-right: none !important; }
  .detail-info-cell:last-child { border-bottom: none; }
  .detail-people { grid-template-columns: 1fr; }
}
</style>
