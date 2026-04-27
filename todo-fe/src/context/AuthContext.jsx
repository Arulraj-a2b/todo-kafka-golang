import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import * as authApi from '../api/auth'

const AuthContext = createContext(null)

function isExpired(token) {
  try {
    const [, payload] = token.split('.')
    const { exp } = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')))
    return !exp || Date.now() >= exp * 1000
  } catch {
    return true
  }
}

function readInitialAuth() {
  const stored = localStorage.getItem('token')
  if (!stored || isExpired(stored)) {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    return { token: null, user: null, bootstrapping: false }
  }
  let user = null
  const raw = localStorage.getItem('user')
  if (raw) {
    try {
      user = JSON.parse(raw)
    } catch {
      user = null
    }
  }
  return { token: stored, user, bootstrapping: true }
}

export function AuthProvider({ children }) {
  const [auth, setAuth] = useState(readInitialAuth)
  const { token, user, bootstrapping } = auth
  const setToken = (next) => setAuth((s) => ({ ...s, token: next }))
  const setUser = (next) => setAuth((s) => ({ ...s, user: next }))
  const setBootstrapping = (next) => setAuth((s) => ({ ...s, bootstrapping: next }))

  const logout = useCallback(() => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    setToken(null)
    setUser(null)
  }, [])

  async function login(email, password) {
    const data = await authApi.login(email, password)
    localStorage.setItem('token', data.token)
    localStorage.setItem('user', JSON.stringify(data.user))
    setToken(data.token)
    setUser(data.user)
  }

  async function register(email, password) {
    const data = await authApi.register(email, password)
    localStorage.setItem('token', data.token)
    localStorage.setItem('user', JSON.stringify(data.user))
    setToken(data.token)
    setUser(data.user)
  }

  useEffect(() => {
    if (!token) return
    let cancelled = false
    authApi
      .me()
      .then((fresh) => {
        if (cancelled) return
        localStorage.setItem('user', JSON.stringify(fresh))
        setUser(fresh)
      })
      .catch(() => {
        if (cancelled) return
        logout()
      })
      .finally(() => {
        if (!cancelled) setBootstrapping(false)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    window.addEventListener('auth:unauthorized', logout)
    return () => window.removeEventListener('auth:unauthorized', logout)
  }, [logout])

  return (
    <AuthContext.Provider value={{ token, user, bootstrapping, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
