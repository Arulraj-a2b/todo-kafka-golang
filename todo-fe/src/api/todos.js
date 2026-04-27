import client, { extractError } from './client'

export async function getTodos() {
  try {
    const { data } = await client.get('/todos')
    if (Array.isArray(data)) return data
    return Array.isArray(data?.todos) ? data.todos : []
  } catch (err) {
    throw extractError(err, 'Failed to load todos')
  }
}

export async function createTodo({ title, status, priority, due_date, tags }) {
  try {
    const { data } = await client.post('/todos', {
      title,
      status,
      priority,
      due_date,
      tags,
    })
    return data
  } catch (err) {
    throw extractError(err, 'Failed to create todo')
  }
}

export async function updateTodo(id, { title, status, priority, due_date, tags }) {
  try {
    const { data } = await client.put(`/todos/${encodeURIComponent(id)}`, {
      title,
      status,
      priority,
      due_date,
      tags,
    })
    return data
  } catch (err) {
    throw extractError(err, 'Failed to update todo')
  }
}

export async function deleteTodo(id) {
  try {
    const { data } = await client.delete(`/todos/${encodeURIComponent(id)}`)
    return data
  } catch (err) {
    throw extractError(err, 'Failed to delete todo')
  }
}

export async function exportTodosCsv() {
  try {
    const resp = await client.get('/todos/export', { responseType: 'blob' })
    return resp.data
  } catch (err) {
    throw extractError(err, 'Failed to export')
  }
}

export async function importTodosCsv(file) {
  try {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await client.post('/todos/import', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  } catch (err) {
    throw extractError(err, 'Failed to import')
  }
}
