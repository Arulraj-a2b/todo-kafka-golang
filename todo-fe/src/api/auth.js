import client, { extractError } from './client'

export async function login(email, password) {
  try {
    const { data } = await client.post('/auth/login', { email, password })
    return data
  } catch (err) {
    throw extractError(err, 'Login failed')
  }
}

export async function register(email, password) {
  try {
    const { data } = await client.post('/auth/register', { email, password })
    return data
  } catch (err) {
    throw extractError(err, 'Registration failed')
  }
}

export async function me() {
  const { data } = await client.get('/auth/me')
  return data.user
}
