import { STATUS, STATUS_LABEL } from '../constants/status'

const STYLES = {
  [STATUS.PENDING]: 'bg-amber-100 text-amber-800 ring-amber-200',
  [STATUS.IN_PROGRESS]: 'bg-blue-100 text-blue-800 ring-blue-200',
  [STATUS.COMPLETED]: 'bg-emerald-100 text-emerald-800 ring-emerald-200',
  [STATUS.DELETED]: 'bg-slate-100 text-slate-600 ring-slate-200',
}

export default function StatusBadge({ status }) {
  const cls = STYLES[status] || 'bg-slate-100 text-slate-700 ring-slate-200'
  const label = STATUS_LABEL[status] || status || 'unknown'
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${cls}`}
    >
      {label}
    </span>
  )
}
