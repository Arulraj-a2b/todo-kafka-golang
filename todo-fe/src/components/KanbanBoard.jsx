import {
  DndContext,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  closestCorners,
} from '@dnd-kit/core'
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable'
import KanbanColumn from './KanbanColumn'
import { COLUMNS, VISIBLE_STATUSES } from '../constants/status'

// Group todos by status. `deleted` items are filtered at the caller level.
function groupByStatus(todos) {
  const groups = Object.fromEntries(VISIBLE_STATUSES.map((s) => [s, []]))
  for (const t of todos) {
    if (groups[t.status]) groups[t.status].push(t)
  }
  return groups
}

export default function KanbanBoard({
  todos,
  onUpdate,
  onDelete,
  updatingIds,
  deletingIds,
}) {
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const grouped = groupByStatus(todos)

  function handleDragEnd(event) {
    const { active, over } = event
    if (!over) return

    const activeId = active.id
    const overId = over.id

    const activeTodo = todos.find((t) => t.id === activeId)
    if (!activeTodo) return

    // Figure out destination column. over.id is either a column id (empty drop
    // zone) or another todo id (dropped onto a card).
    let destColumn
    if (VISIBLE_STATUSES.includes(overId)) {
      destColumn = overId
    } else {
      const overTodo = todos.find((t) => t.id === overId)
      destColumn = overTodo?.status
    }

    if (!destColumn || destColumn === activeTodo.status) return

    onUpdate(activeTodo.id, {
      title: activeTodo.title,
      status: destColumn,
      priority: activeTodo.priority,
      due_date: activeTodo.due_date,
      tags: activeTodo.tags,
    })
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragEnd={handleDragEnd}
    >
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {COLUMNS.map((col) => (
          <KanbanColumn
            key={col.id}
            column={col}
            todos={grouped[col.id] ?? []}
            onUpdate={onUpdate}
            onDelete={onDelete}
            updatingIds={updatingIds}
            deletingIds={deletingIds}
          />
        ))}
      </div>
    </DndContext>
  )
}
