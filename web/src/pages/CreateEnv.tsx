import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { envs } from '../api/envs'
import { templates } from '../api/templates'
import type { Preset, UserEditableField } from '../types/api'

export default function CreateEnv() {
  const navigate = useNavigate()
  const [presets, setPresets] = useState<Preset[]>([])
  const [selectedPreset, setSelectedPreset] = useState<Preset | null>(null)
  const [name, setName] = useState('')
  const [overrides, setOverrides] = useState<Record<string, unknown>>({})
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    templates.list().then((list) => {
      setPresets(list)
      if (list.length > 0) setSelectedPreset(list[0])
    })
  }, [])

  const handlePresetChange = (presetName: string) => {
    const p = presets.find((p) => p.name === presetName) ?? null
    setSelectedPreset(p)
    setOverrides({})
  }

  const handleOverride = (key: string, value: unknown) => {
    setOverrides((prev) => ({ ...prev, [key]: value }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedPreset) return
    setSubmitting(true)
    setError(null)
    try {
      const result = await envs.create({ name, template: selectedPreset.name, overrides })
      navigate(`/envs/${result.name}`)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create env')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Create Environment</h1>
      <form onSubmit={handleSubmit} className="bg-white shadow rounded-lg p-6 space-y-5">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Env Name</label>
          <input
            required
            pattern="[a-z0-9][a-z0-9-]{1,28}[a-z0-9]"
            title="Lowercase letters, numbers, hyphens only (3-30 chars)"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="my-env-name"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Template</label>
          <select
            value={selectedPreset?.name ?? ''}
            onChange={(e) => handlePresetChange(e.target.value)}
            className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            {presets.map((p) => (
              <option key={p.name} value={p.name}>{p.name} — {p.description}</option>
            ))}
          </select>
        </div>

        {selectedPreset && selectedPreset.user_editable.map((field) => (
          <FieldInput
            key={field.key}
            field={field}
            defaultValue={overrides[field.key] ?? field.default}
            onChange={(v) => handleOverride(field.key, v)}
          />
        ))}

        {error && <p className="text-sm text-red-600">{error}</p>}

        <div className="flex gap-3 pt-2">
          <button
            type="submit"
            disabled={submitting}
            className="px-5 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700 disabled:opacity-50"
          >
            {submitting ? 'Creating…' : 'Create Env'}
          </button>
          <button
            type="button"
            onClick={() => navigate('/')}
            className="px-5 py-2 text-sm text-gray-700 border border-gray-300 rounded hover:bg-gray-50"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}

function FieldInput({
  field,
  defaultValue,
  onChange,
}: {
  field: UserEditableField
  defaultValue: unknown
  onChange: (v: unknown) => void
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">{field.label}</label>
      {field.type === 'enum' ? (
        <select
          defaultValue={String(defaultValue ?? '')}
          onChange={(e) => onChange(e.target.value)}
          className="w-full border border-gray-300 rounded px-3 py-2 text-sm"
        >
          {field.options?.map((o) => <option key={o} value={o}>{o}</option>)}
        </select>
      ) : field.type === 'integer' ? (
        <input
          type="number"
          defaultValue={Number(defaultValue ?? field.min ?? 0)}
          min={field.min}
          max={field.max}
          onChange={(e) => onChange(Number(e.target.value))}
          className="w-full border border-gray-300 rounded px-3 py-2 text-sm"
        />
      ) : field.type === 'boolean' ? (
        <input
          type="checkbox"
          defaultChecked={Boolean(defaultValue)}
          onChange={(e) => onChange(e.target.checked)}
          className="h-4 w-4 text-blue-600 border-gray-300 rounded"
        />
      ) : (
        <input
          type="text"
          defaultValue={String(defaultValue ?? '')}
          onChange={(e) => onChange(e.target.value)}
          className="w-full border border-gray-300 rounded px-3 py-2 text-sm"
        />
      )}
    </div>
  )
}
