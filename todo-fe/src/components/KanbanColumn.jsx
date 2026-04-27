import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import TodoCard from './TodoCard'

export default function KanbanColumn({
  column,
  todos,
  onUpdate,
  onDelete,
  updatingIds,
  deletingIds,
}) {
  const { setNodeRef, isOver } = useDroppable({ id: column.id })

  return (
    <section
      className={`flex min-h-[200px] flex-col overflow-hidden rounded-2xl bg-slate-50/60 ring-1 ring-slate-200 ${
        isOver ? 'ring-2 ring-indigo-300' : ''
      }`}
    >
      <div className={`h-1 ${column.accentBar}`} />
      <header
        className={`flex items-center justify-between px-4 py-3 ${column.headerBg}`}
      >
        <h2 className={`text-sm font-semibold ${column.headerText}`}>
          {column.title}
        </h2>
        <span
          className={`rounded-full bg-white/80 px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${column.headerText} ${column.ring}`}
        >
          {todos.length}
        </span>
      </header>

      <div
        ref={setNodeRef}
        className={`flex-1 p-3 transition ${isOver ? 'bg-indigo-50/40' : ''}`}
      >
        <SortableContext
          items={todos.map((t) => t.id)}
          strategy={verticalListSortingStrategy}
        >
          <ul className="space-y-2">
            {todos.map((t) => (
              <TodoCard
                key={t.id}
                todo={t}
                onUpdate={onUpdate}
                onDelete={onDelete}
                updating={updatingIds.has(t.id)}
                deleting={deletingIds.has(t.id)}
              />
            ))}
          </ul>
        </SortableContext>

        {todos.length === 0 && (
          <div className="flex h-24 items-center justify-center rounded-lg border-2 border-dashed border-slate-200 text-xs text-slate-400">
            Drop tasks here
          </div>
        )}
      </div>
    </section>
  )
}
