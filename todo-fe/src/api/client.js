import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  timeout: 60000,
  headers: { 'Content-Type': 'application/json' },
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

client.interceptors.response.use(
  (resp) => resp,
  (err) => {
    if (err?.response?.status === 401) {
      window.dispatchEvent(new CustomEvent('auth:unauthorized'))
    }
    return Promise.reject(err)
  },
)

export function extractError(err, fallback) {
  const msg =
    err?.response?.data?.error ||
    err?.response?.data?.message ||
    err?.message ||
    fallback
  const error = new Error(msg)
  error.status = err?.response?.status
  return error
}

export default client
