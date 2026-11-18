import { useEffect, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { api, type Me } from './api'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Admin from './pages/Admin'

/** `/` — decide where the user belongs based on their session. */
function RootRedirect() {
  const [state, setState] = useState<{ me: Me | null; done: boolean }>({ me: null, done: false })

  useEffect(() => {
    let cancelled = false
    api<Me>('/me')
      .then((me) => {
        if (!cancelled) setState({ me, done: true })
      })
      .catch(() => {
        // 401 (or any failure) — the API helper already sent us to /login on 401.
        if (!cancelled) setState({ me: null, done: true })
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (!state.done) return <div className="loading">Loading…</div>
  if (!state.me) return <Navigate to="/login" replace />
  if (state.me.role === 'admin') return <Navigate to="/admin" replace />
  return <Navigate to="/dashboard" replace />
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<RootRedirect />} />
      <Route path="/login" element={<Login />} />
      <Route path="/dashboard" element={<Dashboard />} />
      <Route path="/admin" element={<Admin />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
