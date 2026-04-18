import { useState } from 'react'
import { envs } from '../api/envs'
import { ImageTagModal } from './ImageTagModal'
import type { Workload } from '../types/api'

interface Props {
  envName: string
  workloads: Workload[]
  onUpdated: () => void
}

interface EditTarget {
  workloadName: string
  container: string
  currentTag: string
}

export function WorkloadsTable({ envName, workloads, onUpdated }: Props) {
  const [editing, setEditing] = useState<EditTarget | null>(null)

  if (workloads.length === 0) {
    return <p className="text-sm text-gray-400 py-4">No workloads found in this namespace.</p>
  }

  const rows = workloads.flatMap((w) =>
    w.containers.map((c) => ({
      kind: w.kind,
      workloadName: w.name,
      readyReplicas: w.ready_replicas,
      desiredReplicas: w.desired_replicas,
      container: c.name,
      image: c.image,
      tag: c.tag,
    }))
  )

  return (
    <>
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {['Kind', 'Name', 'Container', 'Replicas', 'Image', 'Tag', ''].map((h) => (
                <th key={h} className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {rows.map((row, i) => (
              <tr key={i} className="hover:bg-gray-50">
                <td className="px-3 py-2 text-gray-500">{row.kind}</td>
                <td className="px-3 py-2 font-medium text-gray-900">{row.workloadName}</td>
                <td className="px-3 py-2 text-gray-600">{row.container}</td>
                <td className="px-3 py-2 text-gray-600">{row.readyReplicas}/{row.desiredReplicas}</td>
                <td className="px-3 py-2 font-mono text-xs text-gray-600 max-w-xs truncate">{row.image}</td>
                <td className="px-3 py-2 font-mono text-xs text-blue-700">{row.tag}</td>
                <td className="px-3 py-2">
                  <button
                    onClick={() => setEditing({ workloadName: row.workloadName, container: row.container, currentTag: row.tag })}
                    className="text-gray-400 hover:text-gray-700"
                    title="Edit image tag"
                  >
                    ✏️
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {editing && (
        <ImageTagModal
          envName={envName}
          workloadName={editing.workloadName}
          container={editing.container}
          currentTag={editing.currentTag}
          onSave={async (newTag) => {
            await envs.updateImage(envName, {
              workload_name: editing.workloadName,
              container: editing.container,
              new_tag: newTag,
            })
            onUpdated()
          }}
          onClose={() => setEditing(null)}
        />
      )}
    </>
  )
}
