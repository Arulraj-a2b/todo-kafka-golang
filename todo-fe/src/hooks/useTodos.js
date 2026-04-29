import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  createTodo as apiCreate,
  deleteTodo as apiDelete,
  getTodos as apiList,
  updateTodo as apiUpdate,
} from '../api/todos'
import { STATUS } from '../constants/status'

const PAGE_SIZE = 50

function sortTodos(list) {
  return [...list].sort((a, b) => {
    const ta = new Date(a.created_at || 0).getTime()
    const tb = new Date(b.created_at || 0).getTime()
    return tb - ta
  })
}

export default function useTodos() {
  const [todos, setTodos] = useState([])
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState(null)
  const [creating, setCreating] = useState(false)
  const [updatingIds, setUpdatingIds] = useState(() => new Set())
  const [deletingIds, setDeletingIds] = useState(() => new Set())
  const [nextCursor, setNextCursor] = useState('')
  const [hasMore, setHasMore] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const { todos: page, nextCursor: nc, hasMore: hm } = await apiList({ limit: PAGE_SIZE })
      setTodos(sortTodos(page))
      setNextCursor(nc)
      setHasMore(hm)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [])

  const loadMore = useCallback(async () => {
    if (!hasMore || !nextCursor || loadingMore) return
    setLoadingMore(true)
    setError(null)
    try {
      const { todos: page, nextCursor: nc, hasMore: hm } = await apiList({
        limit: PAGE_SIZE,
        cursor: nextCursor,
      })
      setTodos((prev) => sortTodos([...prev, ...page]))
      setNextCursor(nc)
      setHasMore(hm)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoadingMore(false)
    }
  }, [hasMore, nextCursor, loadingMore])

  useEffect(() => {
    load()
  }, [load])

  const create = useCallback(async ({ title, status, priority, due_date, tags }) => {
    setError(null)
    setCreating(true)
    try {
      const created = await apiCreate({ title, status, priority, due_date, tags })
      setTodos((prev) => sortTodos([created, ...prev]))
      return true
    } catch (err) {
      setError(err.message)
      return false
    } finally {
      setCreating(false)
    }
  }, [])

  const update = useCallback(async (id, patch) => {
    setError(null)
    setUpdatingIds((prev) => new Set(prev).add(id))
    let snapshot
    setTodos((prev) => {
      snapshot = prev
      return prev.map((t) => (t.id === id ? { ...t, ...patch } : t))
    })
    try {
      const updated = await apiUpdate(id, patch)
      setTodos((prev) => prev.map((t) => (t.id === id ? { ...t, ...updated } : t)))
      return true
    } catch (err) {
      if (snapshot) setTodos(snapshot)
      setError(err.message)
      return false
    } finally {
      setUpdatingIds((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    }
  }, [])

  const remove = useCallback(async (todo) => {
    setError(null)
    setDeletingIds((prev) => new Set(prev).add(todo.id))
    let snapshot
    setTodos((prev) => {
      snapshot = prev
      return prev.filter((t) => t.id !== todo.id)
    })
    try {
      await apiDelete(todo.id)
      return true
    } catch (err) {
      if (err.status !== 404 && !/not found/i.test(err.message)) {
        if (snapshot) setTodos(snapshot)
      }
      setError(err.message)
      return false
    } finally {
      setDeletingIds((prev) => {
        const next = new Set(prev)
        next.delete(todo.id)
        return next
      })
    }
  }, [])

  const visible = useMemo(
    () => todos.filter((t) => t.status !== STATUS.DELETED),
    [todos],
  )

  const counts = useMemo(() => {
    const c = { total: visible.length }
    for (const t of visible) {
      c[t.status] = (c[t.status] ?? 0) + 1
    }
    return c
  }, [visible])

  return {
    todos: visible,
    counts,
    loading,
    loadingMore,
    hasMore,
    error,
    creating,
    updatingIds,
    deletingIds,
    load,
    loadMore,
    create,
    update,
    remove,
    clearError: () => setError(null),
  }
}
