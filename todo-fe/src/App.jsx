import { useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Header from './components/Header'
import TodoForm from './components/TodoForm'
import KanbanBoard from './components/KanbanBoard'
import Spinner from './components/Spinner'
import ConfirmDialog from './components/ConfirmDialog'
import useTodos from './hooks/useTodos'
import { AuthProvider, useAuth } from './context/AuthContext'
import Login from './pages/Login'
import Register from './pages/Register'
import './App.css'

function FullScreenSpinner() {
  return (
    <div className="flex min-h-screen items-center justify-center">
      <Spinner size="lg" />
    </div>
  )
}

function ProtectedRoute({ children }) {
  const { token, bootstrapping } = useAuth()
  if (bootstrapping) return <FullScreenSpinner />
  if (!token) return <Navigate to="/login" replace />
  return children
}

function PublicOnlyRoute({ children }) {
  const { token, bootstrapping } = useAuth()
  if (bootstrapping) return <FullScreenSpinner />
  if (token) return <Navigate to="/" replace />
  return children
}

function Home() {
  const {
    todos,
    counts,
    loading,
    error,
    creating,
    updatingIds,
    deletingIds,
    load,
    create,
    update,
    remove,
    clearError,
  } = useTodos()

  const [confirmTarget, setConfirmTarget] = useState(null)

  function requestDelete(todo) {
    setConfirmTarget(todo)
  }

  async function handleConfirmDelete() {
    const target = confirmTarget
    setConfirmTarget(null)
    if (target) await remove(target)
  }

  return (
    <div className="min-h-screen px-4 py-8 sm:py-12">
      <main className="mx-auto w-full max-w-6xl">
        <Header counts={counts} onImported={load} />

        <TodoForm onCreate={create} creating={creating} />

        {error && (
          <div className="mb-4 flex items-start justify-between gap-3 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">
            <div className="flex items-start gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="mt-0.5 h-4 w-4 shrink-0">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
              <span>{error}</span>
            </div>
            <div className="flex shrink-0 gap-2">
              <button
                type="button"
                onClick={load}
                className="rounded-md bg-white px-2.5 py-1 text-xs font-medium text-red-700 ring-1 ring-red-200 hover:bg-red-100"
              >
                Retry
              </button>
              <button
                type="button"
                onClick={clearError}
                aria-label="Dismiss error"
                className="rounded-md px-2 py-1 text-xs text-red-700 hover:bg-red-100"
              >
                ✕
              </button>
            </div>
          </div>
        )}

        {loading ? (
          <div className="flex justify-center py-16">
            <Spinner size="lg" />
          </div>
        ) : (
          <KanbanBoard
            todos={todos}
            onUpdate={update}
            onDelete={requestDelete}
            updatingIds={updatingIds}
            deletingIds={deletingIds}
          />
        )}

        <footer className="mt-10 text-center text-xs text-slate-400">
          Connected to <code className="rounded bg-slate-100 px-1.5 py-0.5 text-slate-600">localhost:8000</code> via Vite proxy
        </footer>
      </main>

      <ConfirmDialog
        open={!!confirmTarget}
        title="Delete this todo?"
        message={
          confirmTarget
            ? `"${confirmTarget.title}" will be permanently removed. This cannot be undone.`
            : ''
        }
        confirmLabel="Delete"
        cancelLabel="Cancel"
        destructive
        onConfirm={handleConfirmDelete}
        onCancel={() => setConfirmTarget(null)}
      />
    </div>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route
            path="/login"
            element={
              <PublicOnlyRoute>
                <Login />
              </PublicOnlyRoute>
            }
          />
          <Route
            path="/register"
            element={
              <PublicOnlyRoute>
                <Register />
              </PublicOnlyRoute>
            }
          />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Home />
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
