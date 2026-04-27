import { useRef, useState } from 'react'
import { STATUS, STATUS_LABEL } from '../constants/status'
import { useAuth } from '../context/AuthContext'
import { exportTodosCsv, importTodosCsv } from '../api/todos'
import Spinner from './Spinner'

export default function Header({ counts, onImported }) {
  const { user, logout } = useAuth()
  const fileRef = useRef(null)
  const [exporting, setExporting] = useState(false)
  const [importing, setImporting] = useState(false)
  const [notice, setNotice] = useState(null)

  const total = counts.total ?? 0

  async function handleExport() {
    setNotice(null)
    setExporting(true)
    try {
      const blob = await exportTodosCsv()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'todos.csv'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch (err) {
      setNotice({ type: 'error', text: err.message || 'Export failed' })
    } finally {
      setExporting(false)
    }
  }

  async function handleImportFile(e) {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setNotice(null)
    setImporting(true)
    try {
      const res = await importTodosCsv(file)
      setNotice({
        type: res.failed > 0 ? 'warn' : 'ok',
        text: `Imported ${res.imported}${res.failed ? `, ${res.failed} failed` : ''}.`,
      })
      onImported?.()
    } catch (err) {
      setNotice({ type: 'error', text: err.message || 'Import failed' })
    } finally {
      setImporting(false)
    }
  }

  return (
    <header className="mb-6 space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-md">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-5 w-5">
              <rect x="3" y="3" width="7" height="18" rx="1.5" />
              <rect x="10" y="3" width="7" height="12" rx="1.5" />
              <rect x="17" y="3" width="4" height="7" rx="1" />
            </svg>
          </div>
          <div>
            <h1 className="text-2xl font-bold text-slate-900">Todo Kanban</h1>
            <p className="text-sm text-slate-500">Drag cards between columns to change status.</p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={handleExport}
            disabled={exporting}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50"
          >
            {exporting ? <Spinner size="sm" /> : (
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="7 10 12 15 17 10" />
                <line x1="12" y1="15" x2="12" y2="3" />
              </svg>
            )}
            Export CSV
          </button>

          <button
            type="button"
            onClick={() => fileRef.current?.click()}
            disabled={importing}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50"
          >
            {importing ? <Spinner size="sm" /> : (
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17 8 12 3 7 8" />
                <line x1="12" y1="3" x2="12" y2="15" />
              </svg>
            )}
            Import CSV
          </button>

          <input
            ref={fileRef}
            type="file"
            accept=".csv,text/csv"
            onChange={handleImportFile}
            className="hidden"
          />

          {user?.email && (
            <span className="hidden sm:inline-block rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600 ring-1 ring-slate-200">
              {user.email}
            </span>
          )}

          <button
            type="button"
            onClick={logout}
            className="inline-flex items-center gap-1.5 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-semibold text-white shadow-sm transition hover:bg-slate-900"
          >
            Logout
          </button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="rounded-full bg-white px-3 py-1 font-medium text-slate-700 shadow-sm ring-1 ring-slate-200">
          {total} total
        </span>
        <span className="rounded-full bg-amber-50 px-3 py-1 font-medium text-amber-800 ring-1 ring-amber-200">
          {counts[STATUS.PENDING] ?? 0} {STATUS_LABEL[STATUS.PENDING].toLowerCase()}
        </span>
        <span className="rounded-full bg-blue-50 px-3 py-1 font-medium text-blue-800 ring-1 ring-blue-200">
          {counts[STATUS.IN_PROGRESS] ?? 0} in progress
        </span>
        <span className="rounded-full bg-emerald-50 px-3 py-1 font-medium text-emerald-800 ring-1 ring-emerald-200">
          {counts[STATUS.COMPLETED] ?? 0} {STATUS_LABEL[STATUS.COMPLETED].toLowerCase()}
        </span>
      </div>

      {notice && (
        <div
          className={`rounded-lg px-3 py-2 text-xs ring-1 ${
            notice.type === 'error'
              ? 'bg-red-50 text-red-700 ring-red-200'
              : notice.type === 'warn'
                ? 'bg-amber-50 text-amber-800 ring-amber-200'
                : 'bg-emerald-50 text-emerald-800 ring-emerald-200'
          }`}
        >
          {notice.text}
        </div>
      )}
    </header>
  )
}
