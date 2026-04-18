# React Frontend — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the React SPA for the Env Management Portal: env list, env detail, create env form, workloads table with image tag editing, YAML values editor, on-demand status, and event log. Embed the built output into the Go binary.

**Architecture:** Vite + React + TypeScript SPA. API calls via a typed `fetch` wrapper. No external UI library — Tailwind CSS for styling. Monaco editor for YAML editing. Built output (`web/dist`) embedded into the Go binary via `go:embed`.

**Prerequisite:** Plans 1, 2, and 3 must be complete. The Go backend must be running locally for manual testing.

**Tech Stack:** React 18, TypeScript, Vite, Tailwind CSS, `@monaco-editor/react`, React Router v6

---

## File Map

| File | Responsibility |
|------|---------------|
| `web/package.json` | Dependencies and build scripts |
| `web/vite.config.ts` | Vite config: proxy `/api` and `/auth` to Go backend in dev |
| `web/tsconfig.json` | TypeScript config |
| `web/tailwind.config.js` | Tailwind config |
| `web/index.html` | HTML entry point |
| `web/src/main.tsx` | React entry, router setup |
| `web/src/App.tsx` | Top-level layout, routes |
| `web/src/types/api.ts` | TypeScript types matching Go API responses |
| `web/src/api/client.ts` | Base fetch wrapper with error handling |
| `web/src/api/envs.ts` | Typed API calls for env endpoints |
| `web/src/api/templates.ts` | Typed API calls for template endpoints |
| `web/src/hooks/usePolling.ts` | Generic polling hook |
| `web/src/hooks/useEnvStatus.ts` | On-demand status fetch hook |
| `web/src/pages/EnvList.tsx` | Env list page: table, search, sort, Create button |
| `web/src/pages/CreateEnv.tsx` | Create env form: name, preset, dynamic overrides |
| `web/src/pages/EnvDetail.tsx` | Env detail: metadata, actions, workloads, events |
| `web/src/components/StatusBadge.tsx` | Color-coded status pill |
| `web/src/components/WorkloadsTable.tsx` | Table of k8s workloads with inline edit |
| `web/src/components/ImageTagModal.tsx` | Modal for editing an image tag |
| `web/src/components/ValuesEditor.tsx` | Monaco YAML editor |
| `web/src/components/EventLog.tsx` | Chronological list of portal events |
| `web/src/components/ConfirmDialog.tsx` | Reusable confirm dialog (used for delete) |
| `cmd/portal/main.go` | Add `go:embed web/dist` (modify existing) |
| `internal/server/server.go` | Serve embedded static files for non-API routes (modify) |

---

## Task 1: Vite project setup

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/tailwind.config.js`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`

- [ ] **Step 1: Initialize the Vite project**

```bash
cd web
npm create vite@latest . -- --template react-ts
npm install
```

- [ ] **Step 2: Install dependencies**

```bash
npm install react-router-dom @monaco-editor/react
npm install -D tailwindcss @tailwindcss/vite autoprefixer
npx tailwindcss init
```

- [ ] **Step 3: Configure `web/vite.config.ts`**

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
  },
})
```

- [ ] **Step 4: Configure `web/tailwind.config.js`**

```javascript
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: { extend: {} },
  plugins: [],
}
```

- [ ] **Step 5: Update `web/index.html`**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Env Portal</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 6: Write `web/src/main.tsx`**

```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </React.StrictMode>
)
```

- [ ] **Step 7: Write `web/src/index.css`**

```css
@import "tailwindcss";
```

- [ ] **Step 8: Write `web/src/App.tsx`**

```typescript
import { Routes, Route, Navigate } from 'react-router-dom'
import EnvList from './pages/EnvList'
import EnvDetail from './pages/EnvDetail'
import CreateEnv from './pages/CreateEnv'

export default function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white border-b border-gray-200 px-6 py-3 flex items-center justify-between">
        <a href="/" className="text-lg font-semibold text-gray-900">Env Portal</a>
        <a href="/auth/logout" className="text-sm text-gray-500 hover:text-gray-700">Logout</a>
      </nav>
      <main className="max-w-7xl mx-auto px-6 py-8">
        <Routes>
          <Route path="/" element={<EnvList />} />
          <Route path="/envs/new" element={<CreateEnv />} />
          <Route path="/envs/:name" element={<EnvDetail />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  )
}
```

- [ ] **Step 9: Verify dev server starts**

```bash
cd web && npm run dev
```

Expected: Vite dev server starts on http://localhost:5173 with no errors.

- [ ] **Step 10: Commit**

```bash
cd ..
git add web/
git commit -m "feat: initialize Vite + React + TypeScript + Tailwind project"
```

---

## Task 2: API types and client

**Files:**
- Create: `web/src/types/api.ts`
- Create: `web/src/api/client.ts`
- Create: `web/src/api/envs.ts`
- Create: `web/src/api/templates.ts`

- [ ] **Step 1: Write `web/src/types/api.ts`**

```typescript
export interface EnvSummary {
  name: string
  owner: string
  template: string
  machine_type: string
  zone: string
  external_ip: string
  created_at: string
  job_state?: string
}

export interface EnvStatus {
  status: 'ready' | 'degraded' | 'provisioning' | 'deleting' | 'failed'
  workloads?: Workload[]
}

export interface Workload {
  kind: string
  name: string
  namespace: string
  ready_replicas: number
  desired_replicas: number
  containers: ContainerInfo[]
}

export interface ContainerInfo {
  name: string
  image: string
  tag: string
}

export interface EnvEvent {
  id: number
  env_name: string
  actor_email: string
  action_type: string
  description: string
  outcome: 'success' | 'failure' | 'in_progress'
  log_path?: string
  created_at: string
}

export interface Preset {
  name: string
  description: string
  defaults: {
    machine_type: string
    disk_size_gb: number
    zone: string
    chart_ref: string
  }
  user_editable: UserEditableField[]
}

export interface UserEditableField {
  key: string
  label: string
  type: 'enum' | 'integer' | 'string' | 'boolean'
  options?: string[]
  min?: number
  max?: number
  default?: unknown
}

export interface ChartVersion {
  Ref: string
  SHA: string
  Kind: 'tag' | 'branch'
}

export interface ApiError {
  error: string
}
```

- [ ] **Step 2: Write `web/src/api/client.ts`**

```typescript
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export async function apiFetch<T>(
  path: string,
  options?: RequestInit
): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  })

  if (res.status === 401 || res.status === 302) {
    window.location.href = '/auth/login'
    throw new ApiError(res.status, 'Redirecting to login')
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, body.error ?? res.statusText)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}
```

- [ ] **Step 3: Write `web/src/api/envs.ts`**

```typescript
import { apiFetch } from './client'
import type { EnvSummary, EnvStatus, EnvEvent } from '../types/api'

export const envs = {
  list: () =>
    apiFetch<EnvSummary[]>('/api/envs'),

  get: (name: string) =>
    apiFetch<EnvSummary>(`/api/envs/${name}`),

  status: (name: string) =>
    apiFetch<EnvStatus>(`/api/envs/${name}/status`),

  create: (body: { name: string; template: string; overrides?: Record<string, unknown> }) =>
    apiFetch<EnvSummary>('/api/envs', { method: 'POST', body: JSON.stringify(body) }),

  delete: (name: string) =>
    apiFetch<void>(`/api/envs/${name}`, { method: 'DELETE' }),

  kubeconfig: (name: string) =>
    fetch(`/api/envs/${name}/kubeconfig`).then(async (res) => {
      if (!res.ok) throw new Error('Failed to download kubeconfig')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${name}-kubeconfig.yaml`
      a.click()
      URL.revokeObjectURL(url)
    }),

  editValues: (name: string, values_yaml: string) =>
    apiFetch<{ status: string }>(`/api/envs/${name}/edit-values`, {
      method: 'POST',
      body: JSON.stringify({ values_yaml }),
    }),

  updateImage: (name: string, body: { workload_name: string; container: string; new_tag: string }) =>
    apiFetch<{ status: string }>(`/api/envs/${name}/update-image`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  upgradeChart: (name: string, chart_ref: string) =>
    apiFetch<{ status: string }>(`/api/envs/${name}/upgrade-chart`, {
      method: 'POST',
      body: JSON.stringify({ chart_ref }),
    }),

  resize: (name: string, body: { machine_type?: string; disk_size_gb?: number }) =>
    apiFetch<{ status: string }>(`/api/envs/${name}/resize`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  events: (name: string, limit = 50, offset = 0) =>
    apiFetch<EnvEvent[]>(`/api/envs/${name}/events?limit=${limit}&offset=${offset}`),
}
```

- [ ] **Step 4: Write `web/src/api/templates.ts`**

```typescript
import { apiFetch } from './client'
import type { Preset, ChartVersion } from '../types/api'

export const templates = {
  list: () =>
    apiFetch<Preset[]>('/api/templates'),

  chartVersions: (name: string) =>
    apiFetch<ChartVersion[]>(`/api/templates/${name}/chart-versions`),
}
```

- [ ] **Step 5: Commit**

```bash
git add web/src/types/ web/src/api/
git commit -m "feat: add API types and typed fetch wrappers"
```

---

## Task 3: Hooks

**Files:**
- Create: `web/src/hooks/usePolling.ts`
- Create: `web/src/hooks/useEnvStatus.ts`

- [ ] **Step 1: Write `web/src/hooks/usePolling.ts`**

```typescript
import { useEffect, useRef } from 'react'

export function usePolling(fn: () => void, intervalMs: number, active: boolean) {
  const fnRef = useRef(fn)
  fnRef.current = fn

  useEffect(() => {
    if (!active) return
    const id = setInterval(() => fnRef.current(), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs, active])
}
```

- [ ] **Step 2: Write `web/src/hooks/useEnvStatus.ts`**

```typescript
import { useState, useCallback } from 'react'
import { envs } from '../api/envs'
import type { EnvStatus } from '../types/api'

export function useEnvStatus(envName: string) {
  const [status, setStatus] = useState<EnvStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await envs.status(envName)
      setStatus(data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch status')
    } finally {
      setLoading(false)
    }
  }, [envName])

  return { status, loading, error, refresh: fetch }
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/
git commit -m "feat: add polling and on-demand status hooks"
```

---

## Task 4: Shared components

**Files:**
- Create: `web/src/components/StatusBadge.tsx`
- Create: `web/src/components/ConfirmDialog.tsx`

- [ ] **Step 1: Write `web/src/components/StatusBadge.tsx`**

```typescript
interface Props {
  status: string
}

const colors: Record<string, string> = {
  ready: 'bg-green-100 text-green-800',
  degraded: 'bg-yellow-100 text-yellow-800',
  provisioning: 'bg-blue-100 text-blue-800',
  deleting: 'bg-red-100 text-red-800',
  failed: 'bg-red-100 text-red-800',
  idle: 'bg-gray-100 text-gray-800',
}

export function StatusBadge({ status }: Props) {
  const cls = colors[status] ?? 'bg-gray-100 text-gray-600'
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${cls}`}>
      {status}
    </span>
  )
}
```

- [ ] **Step 2: Write `web/src/components/ConfirmDialog.tsx`**

```typescript
import { useState } from 'react'

interface Props {
  title: string
  description: string
  confirmText: string   // text user must type to confirm
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({ title, description, confirmText, onConfirm, onCancel }: Props) {
  const [input, setInput] = useState('')
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl p-6 max-w-md w-full mx-4">
        <h2 className="text-lg font-semibold text-gray-900 mb-2">{title}</h2>
        <p className="text-sm text-gray-500 mb-4">{description}</p>
        <p className="text-sm text-gray-700 mb-2">
          Type <span className="font-mono font-bold">{confirmText}</span> to confirm:
        </p>
        <input
          className="w-full border border-gray-300 rounded px-3 py-2 text-sm mb-4 focus:outline-none focus:ring-2 focus:ring-red-500"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={confirmText}
        />
        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={input !== confirmText}
            className="px-4 py-2 text-sm text-white bg-red-600 rounded hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/StatusBadge.tsx web/src/components/ConfirmDialog.tsx
git commit -m "feat: add StatusBadge and ConfirmDialog components"
```

---

## Task 5: Env list page

**Files:**
- Create: `web/src/pages/EnvList.tsx`

- [ ] **Step 1: Write `web/src/pages/EnvList.tsx`**

```typescript
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
    .filter((e) => e.name.includes(search) || e.owner.includes(search))
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
```

- [ ] **Step 2: Start the Go backend and the Vite dev server, then verify the list page**

```bash
# Terminal 1: start Go backend
set -a && source .env && set +a && make run

# Terminal 2: start Vite dev server
cd web && npm run dev
```

Open http://localhost:5173 — you should see the Environments table (empty if no envs exist in GCP yet).

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/EnvList.tsx
git commit -m "feat: add env list page with search and sort"
```

---

## Task 6: Create env page

**Files:**
- Create: `web/src/pages/CreateEnv.tsx`

- [ ] **Step 1: Write `web/src/pages/CreateEnv.tsx`**

```typescript
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
      await envs.create({ name, template: selectedPreset.name, overrides })
      navigate(`/envs/${name}`)
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
```

- [ ] **Step 2: Verify in browser**

Navigate to http://localhost:5173/envs/new — the create form should appear with the preset dropdown and dynamic override fields.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/CreateEnv.tsx
git commit -m "feat: add create env page with dynamic preset fields"
```

---

## Task 7: Workloads table and image tag modal

**Files:**
- Create: `web/src/components/WorkloadsTable.tsx`
- Create: `web/src/components/ImageTagModal.tsx`

- [ ] **Step 1: Write `web/src/components/ImageTagModal.tsx`**

```typescript
import { useState } from 'react'

interface Props {
  envName: string
  workloadName: string
  container: string
  currentTag: string
  onSave: (newTag: string) => Promise<void>
  onClose: () => void
}

export function ImageTagModal({ envName, workloadName, container, currentTag, onSave, onClose }: Props) {
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl p-6 max-w-md w-full mx-4">
        <h2 className="text-base font-semibold text-gray-900 mb-1">Update Image Tag</h2>
        <p className="text-sm text-gray-500 mb-4">
          {workloadName} / {container}
        </p>
        <label className="block text-sm font-medium text-gray-700 mb-1">New Tag</label>
        <input
          value={tag}
          onChange={(e) => setTag(e.target.value)}
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
```

- [ ] **Step 2: Write `web/src/components/WorkloadsTable.tsx`**

```typescript
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
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/WorkloadsTable.tsx web/src/components/ImageTagModal.tsx
git commit -m "feat: add workloads table with inline image tag editing"
```

---

## Task 8: Values editor and event log

**Files:**
- Create: `web/src/components/ValuesEditor.tsx`
- Create: `web/src/components/EventLog.tsx`

- [ ] **Step 1: Write `web/src/components/ValuesEditor.tsx`**

```typescript
import { useState } from 'react'
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-3xl mx-4 flex flex-col" style={{ height: '80vh' }}>
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
          <h2 className="text-base font-semibold text-gray-900">Edit Helm Values</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 text-xl leading-none">×</button>
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
          <button onClick={onClose} className="px-4 py-2 text-sm text-gray-700 border border-gray-300 rounded hover:bg-gray-50">
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
```

- [ ] **Step 2: Write `web/src/components/EventLog.tsx`**

```typescript
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
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/ValuesEditor.tsx web/src/components/EventLog.tsx
git commit -m "feat: add ValuesEditor (Monaco) and EventLog components"
```

---

## Task 9: Env detail page

**Files:**
- Create: `web/src/pages/EnvDetail.tsx`

- [ ] **Step 1: Write `web/src/pages/EnvDetail.tsx`**

```typescript
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
      .finally(() => setLoading(false))
    envs.events(name!).then(setEvents)
  }, [name])

  usePolling(loadEnv, 5000, isInFlight)

  const handleDelete = async () => {
    setShowDelete(false)
    await envs.delete(name!)
    navigate('/')
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
        <ValuesEditor
          initialValues={''}
          onSave={async (yaml) => {
            await envs.editValues(name!, yaml)
            setShowValues(false)
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
```

- [ ] **Step 2: Verify in browser**

Open http://localhost:5173/envs/dev-alice (or any env name from your list). Verify metadata loads, action buttons appear, status refresh works.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/EnvDetail.tsx
git commit -m "feat: add env detail page with metadata, status, workloads, actions, and events"
```

---

## Task 10: Go embed integration

**Files:**
- Modify: `cmd/portal/main.go`
- Modify: `internal/server/server.go`
- Update: `Makefile`

- [ ] **Step 1: Build the React app**

```bash
cd web && npm run build && cd ..
```

Expected: `web/dist/` directory created with `index.html` and assets.

- [ ] **Step 2: Add `go:embed` to `cmd/portal/main.go`**

Add to the top of `cmd/portal/main.go`, after the `package main` line:

```go
package main

import "embed"

//go:embed web/dist
var staticFiles embed.FS
```

Pass `staticFiles` to `server.New(...)`:

```go
srv := server.New(cfg, vmClient, database, staticFiles)
```

Update the `New` function signature in `internal/server/server.go`:

```go
func New(cfg *config.Config, vmClient gcp.VMClient, database *db.DB, staticFiles embed.FS) *Server {
```

- [ ] **Step 3: Add static file serving to `internal/server/server.go`**

Add this import:

```go
import (
    "embed"
    "io/fs"
    "net/http"
    ...
)
```

Add at the end of `buildRouter`, before `return r`:

```go
// Serve embedded React SPA for all non-API routes
webDist, _ := fs.Sub(staticFiles, "web/dist")
fileServer := http.FileServer(http.FS(webDist))
r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
    // For client-side routing: if file not found, serve index.html
    _, err := fs.Stat(webDist, r.URL.Path[1:])
    if err != nil {
        r.URL.Path = "/"
    }
    fileServer.ServeHTTP(w, r)
})
```

- [ ] **Step 4: Update `Makefile` to build frontend before Go binary**

```makefile
.PHONY: build run test dev build-frontend

build-frontend:
	cd web && npm run build

build: build-frontend
	go build -o bin/portal ./cmd/portal

run: build
	./bin/portal

test:
	go test ./...

dev-backend:
	go run ./cmd/portal

dev-frontend:
	cd web && npm run dev
```

- [ ] **Step 5: Build the full binary and verify**

```bash
make build
set -a && source .env && set +a && ./bin/portal
```

Open http://localhost:8080 — you should see the portal UI served by the Go binary directly (no Vite dev server needed).

- [ ] **Step 6: Run all tests**

```bash
make test
```

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/portal/main.go internal/server/server.go Makefile web/dist/
git commit -m "feat: embed React SPA into Go binary via go:embed — frontend complete"
```
