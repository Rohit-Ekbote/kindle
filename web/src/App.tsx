import { Routes, Route, Link } from 'react-router-dom'

function Placeholder({ name }: { name: string }) {
  return <p className="text-gray-500 p-8">{name} — coming soon</p>
}

export default function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white border-b border-gray-200 px-6 py-3 flex items-center gap-6">
        <span className="font-semibold text-gray-900">Env Portal</span>
        <Link to="/" className="text-sm text-gray-600 hover:text-gray-900">Environments</Link>
      </nav>
      <main className="px-6 py-8 max-w-7xl mx-auto">
        <Routes>
          <Route path="/" element={<Placeholder name="EnvList" />} />
          <Route path="/envs/new" element={<Placeholder name="CreateEnv" />} />
          <Route path="/envs/:name" element={<Placeholder name="EnvDetail" />} />
        </Routes>
      </main>
    </div>
  )
}
