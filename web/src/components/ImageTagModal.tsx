import { useState } from 'react'

interface Props {
  envName: string
  workloadName: string
  container: string
  currentTag: string
  onSave: (newTag: string) => Promise<void>
  onClose: () => void
}

export function ImageTagModal({ envName: _envName, workloadName, container, currentTag, onSave, onClose }: Props) {
  const [tag, setTag] = useState(currentTag)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSave = async () => {
    setSaving(true)
    setError(null)
    try {
      await onSave(tag)
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update tag')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="image-tag-modal-title"
        className="bg-white rounded-lg shadow-xl p-6 max-w-md w-full mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 id="image-tag-modal-title" className="text-base font-semibold text-gray-900 mb-1">Update Image Tag</h2>
        <p className="text-sm text-gray-500 mb-4">
          {workloadName} / {container}
        </p>
        <label className="block text-sm font-medium text-gray-700 mb-1">New Tag</label>
        <input
          autoFocus
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && tag !== currentTag) handleSave() }}
          className="w-full border border-gray-300 rounded px-3 py-2 text-sm mb-4 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
          placeholder="v1.2.3"
        />
        {error && <p className="text-sm text-red-600 mb-3">{error}</p>}
        <div className="flex justify-end gap-3">
          <button onClick={onClose} className="px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded hover:bg-gray-50">
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving || tag === currentTag}
            className="px-4 py-2 text-sm text-white bg-blue-600 rounded hover:bg-blue-700 disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  )
}
