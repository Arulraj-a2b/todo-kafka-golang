// Single source of truth for status values.
// Backend enum: pending | in_progress | completed | deleted
// (see todo-kafka/models/todo.go)

export const STATUS = Object.freeze({
  PENDING: 'pending',
  IN_PROGRESS: 'in_progress',
  COMPLETED: 'completed',
  DELETED: 'deleted',
})

export const VISIBLE_STATUSES = [
  STATUS.PENDING,
  STATUS.IN_PROGRESS,
  STATUS.COMPLETED,
]

export const STATUS_LABEL = {
  [STATUS.PENDING]: 'Pending',
  [STATUS.IN_PROGRESS]: 'In Progress',
  [STATUS.COMPLETED]: 'Completed',
  [STATUS.DELETED]: 'Deleted',
}

// Column metadata for the Kanban board.
export const COLUMNS = [
  {
    id: STATUS.PENDING,
    title: STATUS_LABEL[STATUS.PENDING],
    headerBg: 'bg-amber-50',
    headerText: 'text-amber-800',
    accentBar: 'bg-gradient-to-r from-amber-400 to-amber-500',
    ring: 'ring-amber-200',
  },
  {
    id: STATUS.IN_PROGRESS,
    title: STATUS_LABEL[STATUS.IN_PROGRESS],
    headerBg: 'bg-blue-50',
    headerText: 'text-blue-800',
    accentBar: 'bg-gradient-to-r from-blue-400 to-blue-500',
    ring: 'ring-blue-200',
  },
  {
    id: STATUS.COMPLETED,
    title: STATUS_LABEL[STATUS.COMPLETED],
    headerBg: 'bg-emerald-50',
    headerText: 'text-emerald-800',
    accentBar: 'bg-gradient-to-r from-emerald-400 to-emerald-500',
    ring: 'ring-emerald-200',
  },
]

export const MAX_TITLE_LENGTH = 200
