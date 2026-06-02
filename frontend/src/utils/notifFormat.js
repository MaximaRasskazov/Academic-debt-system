import iconAppointed   from '../assets/icons/appointed.svg'
import iconChanged     from '../assets/icons/changed.svg'
import iconAppreciated from '../assets/icons/appreciated.svg'

export function notifMeta(kind) {
  switch (kind) {
    case 'retake_scheduled':
    case 'retake_scheduled_teacher':
    case 'retake_request_approved':
    case 'teacher_request_approved':
      return { img: iconAppointed,   bg: 'rgba(110,114,200,.12)', label: 'appointed' }
    case 'retake_change_approved':
    case 'retake_updated':
    case 'retake_updated_teacher':
      return { img: iconChanged,     bg: 'rgba(59,63,224,.10)',   label: 'changed'   }
    case 'retake_cancelled':
    case 'retake_cancelled_teacher':
    case 'retake_change_rejected':
    case 'retake_request_rejected':
    case 'teacher_request_rejected':
      return { img: null,            bg: 'rgba(220,38,38,.08)',   label: 'cancelled' }
    case 'retake_grade_received':
      return { img: iconAppreciated, bg: 'rgba(16,185,129,.10)',  label: 'grade'     }
    default:
      return { img: null,            bg: 'rgba(107,114,128,.10)', label: 'bell'      }
  }
}

export function notifTitle(n) {
  if (n.title) return n.title
  switch (n.kind ?? n.type) {
    case 'retake_scheduled':          return 'Назначена пересдача'
    case 'retake_scheduled_teacher':  return 'Вы назначены на пересдачу'
    case 'retake_updated':            return 'Пересдача перенесена'
    case 'retake_updated_teacher':    return 'Пересдача перенесена'
    case 'retake_cancelled':          return 'Пересдача отменена'
    case 'retake_cancelled_teacher':  return 'Пересдача отменена'
    case 'retake_grade_received':     return 'Выставлена оценка'
    case 'retake_change_approved':    return 'Изменение пересдачи одобрено'
    case 'retake_change_rejected':    return 'Изменение пересдачи отклонено'
    case 'retake_request_approved':   return 'Заявка на пересдачу одобрена'
    case 'retake_request_rejected':   return 'Заявка на пересдачу отклонена'
    case 'teacher_request_approved':  return 'Заявка на преподавателя одобрена'
    case 'teacher_request_rejected':  return 'Заявка на преподавателя отклонена'
    default:                          return 'Уведомление'
  }
}

function fmtDate(iso) {
  if (!iso) return ''
  // Бэк присылает "2026-05-31 17:00" без timezone — добавляем T чтобы
  // браузер не интерпретировал как UTC (иначе getHours даёт UTC+offset)
  const normalized = iso.includes('T') ? iso : iso.replace(' ', 'T')
  const d = new Date(normalized)
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
    + ' в ' + String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0')
}

export function notifBody(n) {
  if (n.body) return n.body
  const p = n.payload || {}
  const when  = p.scheduled_at ? fmtDate(p.scheduled_at) : ''
  const where  = [p.building && `корп. ${p.building}`, p.room && `ауд. ${p.room}`].filter(Boolean).join(', ')
  const reason = p.decision_reason || ''

  switch (n.kind ?? n.type) {
    case 'retake_scheduled':
      return [
        'Вам назначена пересдача.',
        when  && `Дата: ${when}.`,
        where && `Место: ${where}.`,
      ].filter(Boolean).join(' ')

    case 'retake_scheduled_teacher':
      return [
        'Вы назначены преподавателем на пересдачу.',
        when  && `Дата: ${when}.`,
        where && `Место: ${where}.`,
      ].filter(Boolean).join(' ')

    case 'retake_updated':
    case 'retake_updated_teacher':
      return [
        'Детали пересдачи изменены.',
        when  && `Новое время: ${when}.`,
        where && `Место: ${where}.`,
      ].filter(Boolean).join(' ')

    case 'retake_cancelled':
    case 'retake_cancelled_teacher':
      return reason
        ? `Пересдача отменена деканатом. Причина: ${reason}.`
        : 'Пересдача была отменена деканатом.'

    case 'retake_grade_received':
      return p.grade != null
        ? `По результатам пересдачи выставлена оценка: ${p.grade}.`
        : 'По результатам пересдачи выставлена итоговая оценка.'

    case 'retake_change_approved':
      return 'Деканат одобрил вашу заявку на изменение пересдачи. Изменения применены.'

    case 'retake_change_rejected':
      return reason
        ? `Деканат отклонил вашу заявку на изменение пересдачи. Причина: ${reason}.`
        : 'Деканат отклонил вашу заявку на изменение пересдачи.'

    case 'retake_request_approved':
      return 'Деканат одобрил вашу заявку на пересдачу. Пересдача создана.'

    case 'retake_request_rejected':
      return reason
        ? `Деканат отклонил вашу заявку на пересдачу. Причина: ${reason}.`
        : 'Деканат отклонил вашу заявку на пересдачу.'

    case 'teacher_request_approved':
      return 'Ваша заявка на роль преподавателя одобрена деканатом.'

    case 'teacher_request_rejected':
      return reason
        ? `Ваша заявка на роль преподавателя отклонена. Причина: ${reason}.`
        : 'Ваша заявка на роль преподавателя отклонена деканатом.'

    default:
      return ''
  }
}

export function timeAgo(iso) {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  const m = Math.floor(diff / 60000)
  if (m < 1)  return 'только что'
  if (m < 60) return `${m} мин назад`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h} ч назад`
  const d = Math.floor(h / 24)
  if (d < 7)  return `${d} д назад`
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: 'short' })
}
