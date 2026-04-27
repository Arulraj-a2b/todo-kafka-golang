import { MAX_TITLE_LENGTH, VISIBLE_STATUSES } from '../constants/status'
import { PRIORITY_VALUES } from '../constants/priority'

export function validateTitle(raw) {
  const trimmed = (raw ?? '').trim()
  if (trimmed.length === 0) return 'Title is required'
  if (trimmed.length > MAX_TITLE_LENGTH) {
    return `Title must be ${MAX_TITLE_LENGTH} characters or fewer`
  }
  return null
}

export function validateStatus(value) {
  if (!VISIBLE_STATUSES.includes(value)) return 'Select a valid status'
  return null
}

export function validatePriority(value) {
  if (!value) return null
  if (!PRIORITY_VALUES.includes(value)) return 'Select a valid priority'
  return null
}

export function validateTodo({ title, status, priority }) {
  const errors = {}
  const titleErr = validateTitle(title)
  if (titleErr) errors.title = titleErr
  const statusErr = validateStatus(status)
  if (statusErr) errors.status = statusErr
  const priorityErr = validatePriority(priority)
  if (priorityErr) errors.priority = priorityErr
  return { valid: Object.keys(errors).length === 0, errors }
}

export function parseTagsInput(raw) {
  if (!raw) return []
  return raw
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)
}

export function tagsToInput(tags) {
  if (!Array.isArray(tags)) return ''
  return tags.join(', ')
}

export function dueDateToInput(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function inputToDueDate(str) {
  if (!str) return null
  const d = new Date(str)
  if (Number.isNaN(d.getTime())) return null
  return d.toISOString()
}
