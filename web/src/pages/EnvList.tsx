import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { envs } from '../api/envs'
import { StatusBadge } from '../components/StatusBadge'
import type { EnvSummary } from '../types/api'

type SortKey = keyof Pick<EnvSummary, 'name' | 'owner' | 'machine_type' | 'zone' | 'created_at'>

export default function EnvList() {
  const [list, setList] = useState<EnvSummary[]>([])
  const [search, setSearch] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('created_at')
  const [sortAsc, setSortAsc] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    envs.list()
      .then(setList)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  const filtered = list
    .filter((e) => {
      const q = search.toLowerCase()
      return e.name.toLowerCase().includes(q) || e.owner.toLowerCase().includes(q)
    })
    .sort((a, b) => {
      const av = a[sortKey] ?? ''
      const bv = b[sortKey] ?? ''
      return sortAsc
        ? String(av).localeCompare(String(bv))
        : String(bv).localeCompare(String(av))
    })

  const toggleSort = (key: SortKey) => {
    if (key === sortKey) setSortAsc((v) => !v)
    else { setSortKey(key); setSortAsc(true) }
  }

  const th = (label: string, key: SortKey) => (
    <th
      onClick={() => toggleSort(key)}
      className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:bg-gray-100 select-none"
    >
      {label} {sortKey === key ? (sortAsc ? '↑' : '↓') : ''}
    </th>
  )

  if (loading) return <p className="text-gray-500">Loading environments…</p>
  if (error) return <p className="text-red-500">Error: {error}</p>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Environments</h1>
        <button
          onClick={() => navigate('/envs/new')}
          className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700"
        >
          + Create Env
        </button>
      </div>

      <input
        type="text"
        placeholder="Search by name or owner…"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="mb-4 w-full max-w-sm border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
      />

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              {th('Name', 'name')}
              {th('Owner', 'owner')}
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Template</th>
              {th('Machine Type', 'machine_type')}
              {th('Zone', 'zone')}
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
              {th('Created', 'created_at')}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {filtered.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-8 text-center text-gray-400">No environments found</td></tr>
            )}
            {filtered.map((env) => (
              <tr key={env.name} className="hover:bg-gray-50 cursor-pointer" onClick={() => navigate(`/envs/${env.name}`)}>
                <td className="px-4 py-3 text-sm font-medium text-blue-600">
                  <Link to={`/envs/${env.name}`} onClick={(e) => e.stopPropagation()}>{env.name}</Link>
                </td>
                <td className="px-4 py-3 text-sm text-gray-600">{env.owner}</td>
                <td className="px-4 py-3 text-sm text-gray-600">{env.template}</td>
                <td className="px-4 py-3 text-sm text-gray-600">{env.machine_type}</td>
                <td className="px-4 py-3 text-sm text-gray-600">{env.zone}</td>
                <td className="px-4 py-3">
                  {env.job_state
                    ? <StatusBadge status={env.job_state} />
                    : <span className="text-xs text-gray-400">—</span>}
                </td>
                <td className="px-4 py-3 text-sm text-gray-500">
                  {new Date(env.created_at).toLocaleDateString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
