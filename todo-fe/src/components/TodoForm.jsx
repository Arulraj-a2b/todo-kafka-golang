import { useState } from 'react'
import Spinner from './Spinner'
import {
  STATUS,
  STATUS_LABEL,
  VISIBLE_STATUSES,
  MAX_TITLE_LENGTH,
} from '../constants/status'
import { PRIORITY, PRIORITY_LABEL, PRIORITY_VALUES } from '../constants/priority'
import {
  validateTodo,
  parseTagsInput,
  inputToDueDate,
} from '../utils/validate'

export default function TodoForm({ onCreate, creating }) {
  const [title, setTitle] = useState('')
  const [status, setStatus] = useState(STATUS.PENDING)
  const [priority, setPriority] = useState(PRIORITY.MEDIUM)
  const [dueDate, setDueDate] = useState('')
  const [tagsText, setTagsText] = useState('')
  const [errors, setErrors] = useState({})
  const [touched, setTouched] = useState(false)

  const { valid } = validateTodo({ title, status, priority })

  function resetForm() {
    setTitle('')
    setStatus(STATUS.PENDING)
    setPriority(PRIORITY.MEDIUM)
    setDueDate('')
    setTagsText('')
    setTouched(false)
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setTouched(true)
    const { valid: v, errors: errs } = validateTodo({ title, status, priority })
    if (!v) {
      setErrors(errs)
      return
    }
    setErrors({})
    const ok = await onCreate({
      title: title.trim(),
      status,
      priority,
      due_date: inputToDueDate(dueDate),
      tags: parseTagsInput(tagsText),
    })
    if (ok) resetForm()
  }

  function onTitleChange(e) {
    setTitle(e.target.value)
    if (touched) {
      const { errors: errs } = validateTodo({ title: e.target.value, status, priority })
      setErrors(errs)
    }
  }

  const disabled = creating || (touched && !valid)

  return (
    <form
      onSubmit={handleSubmit}
      className="mb-6 rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200"
      noValidate
    >
      <div className="flex flex-col gap-3">
        <div>
          <input
            type="text"
            value={title}
            onChange={onTitleChange}
            onBlur={() => setTouched(true)}
            placeholder="What needs to be done?"
            disabled={creating}
            maxLength={MAX_TITLE_LENGTH + 50}
            className={`w-full rounded-lg border bg-slate-50 px-4 py-2.5 text-sm text-slate-900 placeholder-slate-400 focus:bg-white focus:outline-none focus:ring-2 disabled:opacity-60 ${
              errors.title
                ? 'border-red-300 focus:border-red-500 focus:ring-red-200'
                : 'border-slate-200 focus:border-indigo-500 focus:ring-indigo-200'
            }`}
            aria-invalid={!!errors.title}
          />
          {errors.title && (
            <p className="mt-1 text-xs text-red-600">{errors.title}</p>
          )}
          <p className="mt-1 text-right text-[10px] text-slate-400">
            {title.trim().length}/{MAX_TITLE_LENGTH}
          </p>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            disabled={creating}
            aria-label="Status"
            className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-900 focus:border-indigo-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200 disabled:opacity-60"
          >
            {VISIBLE_STATUSES.map((s) => (
              <option key={s} value={s}>{STATUS_LABEL[s]}</option>
            ))}
          </select>

          <select
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
            disabled={creating}
            aria-label="Priority"
            className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-900 focus:border-indigo-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200 disabled:opacity-60"
          >
            {PRIORITY_VALUES.map((p) => (
              <option key={p} value={p}>{PRIORITY_LABEL[p]} priority</option>
            ))}
          </select>

          <input
            type="date"
            value={dueDate}
            onChange={(e) => setDueDate(e.target.value)}
            disabled={creating}
            aria-label="Due date"
            className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-900 focus:border-indigo-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200 disabled:opacity-60"
          />
        </div>

        <input
          type="text"
          value={tagsText}
          onChange={(e) => setTagsText(e.target.value)}
          placeholder="Tags (comma separated, e.g. work, urgent)"
          disabled={creating}
          className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-900 placeholder-slate-400 focus:border-indigo-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200 disabled:opacity-60"
        />

        <button
          type="submit"
          disabled={disabled}
          className="inline-flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-indigo-500 to-purple-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:from-indigo-600 hover:to-purple-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {creating ? (
            <>
              <Spinner size="sm" className="border-white" />
              <span>Adding…</span>
            </>
          ) : (
            <>
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
              <span>Add Todo</span>
            </>
          )}
        </button>
      </div>
      {creating && (
        <p className="mt-2 text-xs text-slate-500">
          Publishing to Kafka and waiting for confirmation — this may take a few seconds.
        </p>
      )}
    </form>
  )
}
