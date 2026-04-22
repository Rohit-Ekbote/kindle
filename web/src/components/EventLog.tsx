import type { EnvEvent } from '../types/api'

interface Props {
  events: EnvEvent[]
}

const outcomeColor: Record<string, string> = {
  success: 'text-green-600',
  failure: 'text-red-600',
  in_progress: 'text-blue-600',
}

export function EventLog({ events }: Props) {
  if (events.length === 0) {
    return <p className="text-sm text-gray-400 py-4">No events recorded yet.</p>
  }
  return (
    <ol className="space-y-3">
      {events.map((e) => (
        <li key={e.id} className="flex gap-4 text-sm">
          <span className="text-gray-400 shrink-0 w-36 font-mono text-xs pt-0.5">
            {new Date(e.created_at).toLocaleString()}
          </span>
          <div>
            <span className="text-gray-900">{e.description}</span>
            {' '}
            <span className={`font-medium ${outcomeColor[e.outcome] ?? 'text-gray-500'}`}>
              [{e.outcome}]
            </span>
            <div className="text-xs text-gray-400 mt-0.5">{e.actor_email}</div>
          </div>
        </li>
      ))}
    </ol>
  )
}
