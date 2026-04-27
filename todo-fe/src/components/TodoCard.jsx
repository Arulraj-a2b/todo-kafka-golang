import { useEffect, useRef, useState } from 'react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import StatusBadge from './StatusBadge'
import Spinner from './Spinner'
import {
  MAX_TITLE_LENGTH,
  STATUS_LABEL,
  VISIBLE_STATUSES,
} from '../constants/status'
import {
  PRIORITY,
  PRIORITY_LABEL,
  PRIORITY_STYLES,
  PRIORITY_VALUES,
} from '../constants/priority'
import {
  validateTodo,
  parseTagsInput,
  tagsToInput,
  inputToDueDate,
  dueDateToInput,
} from '../utils/validate'

function formatDate(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const diffMs = Date.now() - d.getTime()
  const sec = Math.floor(diffMs / 1000)
  if (sec < 60) return 'just now'
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  const day = Math.floor(hr / 24)
  if (day < 7) return `${day}d ago`
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function formatDueDate(iso) {
  if (!iso) return null
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return null
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const due = new Date(d)
  due.setHours(0, 0, 0, 0)
  const isPast = due.getTime() < today.getTime()
  return {
    label: d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }),
    isPast,
  }
}

export default function TodoCard({
  todo,
  onUpdate,
  onDelete,
  updating,
  deleting,
}) {
  const [mode, setMode] = useState('view')
  const [title, setTitle] = useState(todo.title)
  const [status, setStatus] = useState(todo.status)
  const [priority, setPriority] = useState(todo.priority || PRIORITY.MEDIUM)
  const [dueDate, setDueDate] = useState(dueDateToInput(todo.due_date))
  const [tagsText, setTagsText] = useState(tagsToInput(todo.tags))
  const [errors, setErrors] = useState({})
  const inputRef = useRef(null)

  const isBusy = updating || deleting
  const dragDisabled = mode === 'edit' || isBusy

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: todo.id, disabled: dragDisabled })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  useEffect(() => {
    if (mode === 'edit') inputRef.current?.focus()
  }, [mode])

  function startEdit() {
    setTitle(todo.title)
    setStatus(todo.status)
    setPriority(todo.priority || PRIORITY.MEDIUM)
    setDueDate(dueDateToInput(todo.due_date))
    setTagsText(tagsToInput(todo.tags))
    setErrors({})
    setMode('edit')
  }

  function cancelEdit() {
    setMode('view')
    setErrors({})
  }

  async function saveEdit(e) {
    e?.preventDefault?.()
    const { valid, errors: errs } = validateTodo({ title, status, priority })
    if (!valid) {
      setErrors(errs)
      return
    }
    const ok = await onUpdate(todo.id, {
      title: title.trim(),
      status,
      priority,
      due_date: inputToDueDate(dueDate),
      tags: parseTagsInput(tagsText),
    })
    if (ok) setMode('view')
  }

  function onEditKeyDown(e) {
    if (e.key === 'Escape') {
      e.preventDefault()
      cancelEdit()
    }
  }

  const due = formatDueDate(todo.due_date)
  const priorityStyle = PRIORITY_STYLES[todo.priority] || PRIORITY_STYLES[PRIORITY.MEDIUM]
  const tags = Array.isArray(todo.tags) ? todo.tags : []

  return (
    <li
      ref={setNodeRef}
      style={style}
      className={`group relative rounded-xl bg-white p-3 shadow-sm ring-1 ring-slate-200 transition hover:shadow-md ${
        isDragging ? 'shadow-lg' : ''
      }`}
    >
      {mode === 'view' ? (
        <div className="flex items-start gap-2">
          <button
            type="button"
            {...attributes}
            {...listeners}
            aria-label="Drag to reorder"
            disabled={dragDisabled}
            className={`mt-0.5 shrink-0 rounded p-1 text-slate-300 transition hover:text-slate-500 ${
              dragDisabled ? 'cursor-not-allowed opacity-40' : 'cursor-grab active:cursor-grabbing'
            }`}
          >
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" className="h-4 w-4">
              <circle cx="9" cy="6" r="1.5" />
              <circle cx="15" cy="6" r="1.5" />
              <circle cx="9" cy="12" r="1.5" />
              <circle cx="15" cy="12" r="1.5" />
              <circle cx="9" cy="18" r="1.5" />
              <circle cx="15" cy="18" r="1.5" />
            </svg>
          </button>

          <div className="min-w-0 flex-1">
            <p
              className={`break-words text-sm font-medium ${
                todo.status === 'completed' ? 'line-through text-slate-400' : 'text-slate-900'
              }`}
              title={todo.title}
            >
              {todo.title}
            </p>

            <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs text-slate-500">
              <StatusBadge status={todo.status} />
              {todo.priority && (
                <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium ring-1 ${priorityStyle}`}>
                  {PRIORITY_LABEL[todo.priority] ?? todo.priority}
                </span>
              )}
              {due && (
                <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium ring-1 ${
                  due.isPast
                    ? 'bg-red-50 text-red-700 ring-red-200'
                    : 'bg-slate-50 text-slate-600 ring-slate-200'
                }`}>
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3 w-3">
                    <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
                    <line x1="16" y1="2" x2="16" y2="6" />
                    <line x1="8" y1="2" x2="8" y2="6" />
                    <line x1="3" y1="10" x2="21" y2="10" />
                  </svg>
                  {due.label}
                </span>
              )}
              <span>•</span>
              <span title={todo.created_at}>{formatDate(todo.created_at)}</span>
            </div>

            {tags.length > 0 && (
              <div className="mt-1.5 flex flex-wrap gap-1">
                {tags.map((tag) => (
                  <span
                    key={tag}
                    className="inline-flex items-center rounded bg-indigo-50 px-1.5 py-0.5 text-[10px] font-medium text-indigo-700 ring-1 ring-indigo-200"
                  >
                    #{tag}
                  </span>
                ))}
              </div>
            )}
          </div>

          <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition group-hover:opacity-100 focus-within:opacity-100">
            <button
              type="button"
              onClick={startEdit}
              disabled={isBusy}
              aria-label={`Edit ${todo.title}`}
              className="rounded-md p-1.5 text-slate-400 transition hover:bg-indigo-50 hover:text-indigo-600 disabled:opacity-40"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4">
                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
              </svg>
            </button>
            <button
              type="button"
              onClick={() => onDelete(todo)}
              disabled={isBusy}
              aria-label={`Delete ${todo.title}`}
              className="rounded-md p-1.5 text-slate-400 transition hover:bg-red-50 hover:text-red-600 disabled:opacity-40"
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4">
                <polyline points="3 6 5 6 21 6" />
                <path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6" />
                <path d="M10 11v6M14 11v6" />
                <path d="M9 6V4a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" />
              </svg>
            </button>
          </div>
        </div>
      ) : (
        <form onSubmit={saveEdit} onKeyDown={onEditKeyDown} noValidate className="space-y-2">
          <input
            ref={inputRef}
            type="text"
            value={title}
            onChange={(e) => {
              setTitle(e.target.value)
              if (errors.title) {
                const { errors: errs } = validateTodo({ title: e.target.value, status, priority })
                setErrors(errs)
              }
            }}
            disabled={updating}
            maxLength={MAX_TITLE_LENGTH + 50}
            aria-invalid={!!errors.title}
            className={`w-full rounded-md border bg-white px-2 py-1.5 text-sm text-slate-900 focus:outline-none focus:ring-2 ${
              errors.title
                ? 'border-red-300 focus:border-red-500 focus:ring-red-200'
                : 'border-slate-300 focus:border-indigo-500 focus:ring-indigo-200'
            }`}
          />
          {errors.title && <p className="text-xs text-red-600">{errors.title}</p>}

          <div className="grid grid-cols-2 gap-2">
            <select
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              disabled={updating}
              className="rounded-md border border-slate-300 bg-white px-2 py-1.5 text-xs text-slate-900 focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200"
            >
              {VISIBLE_STATUSES.map((s) => (
                <option key={s} value={s}>{STATUS_LABEL[s]}</option>
              ))}
            </select>

            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value)}
              disabled={updating}
              className="rounded-md border border-slate-300 bg-white px-2 py-1.5 text-xs text-slate-900 focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200"
            >
              {PRIORITY_VALUES.map((p) => (
                <option key={p} value={p}>{PRIORITY_LABEL[p]}</option>
              ))}
            </select>
          </div>

          <input
            type="date"
            value={dueDate}
            onChange={(e) => setDueDate(e.target.value)}
            disabled={updating}
            className="w-full rounded-md border border-slate-300 bg-white px-2 py-1.5 text-xs text-slate-900 focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200"
          />

          <input
            type="text"
            value={tagsText}
            onChange={(e) => setTagsText(e.target.value)}
            placeholder="tags, comma separated"
            disabled={updating}
            className="w-full rounded-md border border-slate-300 bg-white px-2 py-1.5 text-xs text-slate-900 focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200"
          />

          <div className="flex justify-end gap-1.5">
            <button
              type="button"
              onClick={cancelEdit}
              disabled={updating}
              className="rounded-md bg-slate-100 px-3 py-1 text-xs font-medium text-slate-700 transition hover:bg-slate-200 disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={updating}
              className="inline-flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1 text-xs font-semibold text-white transition hover:bg-indigo-700 disabled:opacity-50"
            >
              {updating && <Spinner size="sm" className="border-white" />}
              Save
            </button>
          </div>
        </form>
      )}

      {isBusy && mode === 'view' && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center rounded-xl bg-white/60">
          <Spinner size="sm" />
        </div>
      )}
    </li>
  )
}
