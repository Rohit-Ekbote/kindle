import { useState, useEffect } from 'react'
import Editor from '@monaco-editor/react'

interface Props {
  initialValues: string
  onSave: (values: string) => Promise<void>
  onClose: () => void
}

export function ValuesEditor({ initialValues, onSave, onClose }: Props) {
  const [values, setValues] = useState(initialValues)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const tryClose = () => { if (!saving) onClose() }

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') tryClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [saving]) // re-bind when saving changes so tryClose captures latest value

  const handleSave = async () => {
    setSaving(true)
    setError(null)
    try {
      await onSave(values)
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save values')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={tryClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Edit Helm Values"
        className="bg-white rounded-lg shadow-xl w-full max-w-3xl mx-4 flex flex-col"
        style={{ height: '80vh' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
          <h2 className="text-base font-semibold text-gray-900">Edit Helm Values</h2>
          <button onClick={tryClose} className="text-gray-400 hover:text-gray-600 text-xl leading-none">×</button>
        </div>
        <div className="flex-1 overflow-hidden">
          <Editor
            language="yaml"
            value={values}
            onChange={(v) => setValues(v ?? '')}
            options={{ minimap: { enabled: false }, fontSize: 13, wordWrap: 'on' }}
          />
        </div>
        {error && <p className="text-sm text-red-600 px-5 py-2">{error}</p>}
        <div className="flex justify-end gap-3 px-5 py-3 border-t border-gray-200">
          <button onClick={tryClose} className="px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded hover:bg-gray-50">
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-4 py-2 text-sm text-white bg-blue-600 rounded hover:bg-blue-700 disabled:opacity-50"
          >
            {saving ? 'Applying…' : 'Apply'}
          </button>
        </div>
      </div>
    </div>
  )
}
