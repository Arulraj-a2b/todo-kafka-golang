export const PRIORITY = Object.freeze({
  LOW: 'low',
  MEDIUM: 'medium',
  HIGH: 'high',
})

export const PRIORITY_VALUES = [PRIORITY.LOW, PRIORITY.MEDIUM, PRIORITY.HIGH]

export const PRIORITY_LABEL = {
  [PRIORITY.LOW]: 'Low',
  [PRIORITY.MEDIUM]: 'Medium',
  [PRIORITY.HIGH]: 'High',
}

export const PRIORITY_STYLES = {
  [PRIORITY.LOW]: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  [PRIORITY.MEDIUM]: 'bg-amber-50 text-amber-700 ring-amber-200',
  [PRIORITY.HIGH]: 'bg-red-50 text-red-700 ring-red-200',
}
