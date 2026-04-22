import { lazy, Suspense } from 'react'
import { Routes, Route, Link } from 'react-router-dom'

const EnvList = lazy(() => import('./pages/EnvList'))
const CreateEnv = lazy(() => import('./pages/CreateEnv'))
const EnvDetail = lazy(() => import('./pages/EnvDetail'))

export default function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white border-b border-gray-200 px-6 py-3 flex items-center gap-6">
        <span className="font-semibold text-gray-900">Env Portal</span>
        <Link to="/" className="text-sm text-gray-600 hover:text-gray-900">Environments</Link>
      </nav>
      <main className="px-6 py-8 max-w-7xl mx-auto">
        <Suspense fallback={<p className="text-gray-400">Loading…</p>}>
          <Routes>
            <Route path="/" element={<EnvList />} />
            <Route path="/envs/new" element={<CreateEnv />} />
            <Route path="/envs/:name" element={<EnvDetail />} />
          </Routes>
        </Suspense>
      </main>
    </div>
  )
}
