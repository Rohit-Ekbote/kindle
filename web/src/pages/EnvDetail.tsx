import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { envs } from '../api/envs'
import { StatusBadge } from '../components/StatusBadge'
import { WorkloadsTable } from '../components/WorkloadsTable'
import { ValuesEditor } from '../components/ValuesEditor'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { EventLog } from '../components/EventLog'
import { useEnvStatus } from '../hooks/useEnvStatus'
import { usePolling } from '../hooks/usePolling'
import type { EnvSummary, EnvEvent } from '../types/api'

export default function EnvDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const [env, setEnv] = useState<EnvSummary | null>(null)
  const [events, setEvents] = useState<EnvEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [showValues, setShowValues] = useState(false)
  const [showDelete, setShowDelete] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const { status, loading: statusLoading, refresh: refreshStatus } = useEnvStatus(name!)

  const isInFlight = env?.job_state === 'provisioning' || env?.job_state === 'deleting'

  const loadEnv = () => {
    envs.get(name!).then(setEnv).catch(console.error)
    envs.events(name!).then(setEvents).catch(console.error)
  }

  useEffect(() => {
    envs.get(name!)
      .then(setEnv)
      .catch((e) => setActionError(e instanceof Error ? e.message : 'Failed to load environment'))
      .finally(() => setLoading(false))
    envs.events(name!).then(setEvents).catch(console.error)
  }, [name])

  usePolling(loadEnv, 5000, isInFlight)

  const handleDelete = async () => {
    setShowDelete(false)
    setActionError(null)
    try {
      await envs.delete(name!)
      navigate('/')
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Failed to delete env')
    }
  }

  const handleDownloadKubeconfig = () => envs.kubeconfig(name!)

  if (loading) return <p className="text-gray-500">Loading…</p>
  if (!env) return <p className="text-red-500">Environment not found.</p>

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{env.name}</h1>
          <p className="text-sm text-gray-500 mt-1">Owner: {env.owner} · Template: {env.template}</p>
        </div>
        <div className="flex gap-2 flex-wrap justify-end">
          <button onClick={handleDownloadKubeconfig} className="px-3 py-1.5 text-sm border border-gray-300 rounded hover:bg-gray-50">
            Download kubeconfig
          </button>
          <button onClick={() => setShowValues(true)} className="px-3 py-1.5 text-sm border border-gray-300 rounded hover:bg-gray-50">
            Edit values
          </button>
          <button
            onClick={() => setShowDelete(true)}
            className="px-3 py-1.5 text-sm text-red-600 border border-red-300 rounded hover:bg-red-50"
          >
            Delete env
          </button>
        </div>
      </div>

      {actionError && (
        <div className="bg-red-50 border border-red-200 rounded px-4 py-3 text-sm text-red-700">{actionError}</div>
      )}

      {/* Metadata */}
      <section className="bg-white shadow rounded-lg p-5">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-4">VM Details</h2>
        <dl className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          {[
            ['Machine Type', env.machine_type],
            ['Zone', env.zone],
            ['External IP', env.external_ip],
            ['Created', new Date(env.created_at).toLocaleString()],
          ].map(([label, value]) => (
            <div key={label}>
              <dt className="text-gray-500">{label}</dt>
              <dd className="font-medium text-gray-900 mt-0.5">{value || '—'}</dd>
            </div>
          ))}
        </dl>
      </section>

      {/* Status */}
      <section className="bg-white shadow rounded-lg p-5">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide">Status</h2>
          <button
            onClick={refreshStatus}
            disabled={statusLoading}
            className="text-sm text-blue-600 hover:underline disabled:opacity-50"
          >
            {statusLoading ? 'Fetching…' : 'Refresh Status'}
          </button>
        </div>
        {status ? (
          <StatusBadge status={status.status} />
        ) : (
          <p className="text-sm text-gray-400">Click Refresh Status to fetch live status.</p>
        )}
        {isInFlight && (
          <div className="mt-2">
            <StatusBadge status={env.job_state!} />
            <span className="ml-2 text-xs text-gray-400">Polling for updates…</span>
          </div>
        )}
      </section>

      {/* Workloads */}
      <section className="bg-white shadow rounded-lg p-5">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-4">Workloads</h2>
        {status?.workloads ? (
          <WorkloadsTable
            envName={name!}
            workloads={status.workloads}
            onUpdated={refreshStatus}
          />
        ) : (
          <p className="text-sm text-gray-400">Refresh status to view workloads.</p>
        )}
      </section>

      {/* Event Log */}
      <section className="bg-white shadow rounded-lg p-5">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-4">
          Event Log
          <span className="text-xs text-gray-400 ml-2 normal-case font-normal">portal-initiated actions only</span>
        </h2>
        <EventLog events={events} />
      </section>

      {/* Modals */}
      {showValues && (
        // initialValues is empty: no GET-values API endpoint exists; operator applies a full values overlay
        <ValuesEditor
          initialValues={''}
          onSave={async (yaml) => {
            await envs.editValues(name!, yaml)
            // ValuesEditor calls onClose() itself on success; no need to setShowValues here
          }}
          onClose={() => setShowValues(false)}
        />
      )}

      {showDelete && (
        <ConfirmDialog
          title="Delete Environment"
          description="This will permanently destroy the VM, disk, and DNS record. This cannot be undone."
          confirmText={name!}
          onConfirm={handleDelete}
          onCancel={() => setShowDelete(false)}
        />
      )}
    </div>
  )
}
