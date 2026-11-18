import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Role } from '../api'

const MEMBER_ROLE_OPTIONS: { value: Role; label: string }[] = [
  { value: 'executive', label: 'Executive' },
  { value: 'supervisor', label: 'Supervisor' },
  { value: 'district_coordinator', label: 'District Coordinator' },
  { value: 'regional_coordinator', label: 'Regional Coordinator' },
]

export default function Login() {
  const navigate = useNavigate()
  const [role, setRole] = useState<Role>('executive')
  const [id, setId] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const isAdmin = role === 'admin'

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (isAdmin) {
        await api('/admin/login', { method: 'POST', body: { username, password } })
      } else {
        await api(`/login/${role}`, { method: 'POST', body: { id: Number(id), password } })
      }
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
      setBusy(false)
    }
  }

  return (
    <div className="login-wrap">
      <div className="login-card">
        <h1>Payroll</h1>
        <p className="login-sub">Sign in to view your payouts.</p>

        <form onSubmit={handleSubmit}>
          <label className="field">
            <span>Role</span>
            <select value={role} onChange={(e) => setRole(e.target.value as Role)}>
              {MEMBER_ROLE_OPTIONS.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
              <option value="admin">Admin</option>
            </select>
          </label>

          {isAdmin ? (
            <label className="field">
              <span>Username</span>
              <input
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                required
              />
            </label>
          ) : (
            <label className="field">
              <span>ID</span>
              <input
                type="number"
                value={id}
                onChange={(e) => setId(e.target.value)}
                autoComplete="username"
                required
                min={1}
              />
            </label>
          )}

          <label className="field">
            <span>Password</span>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </label>

          {error && <div className="error">{error}</div>}

          <button className="btn" type="submit" disabled={busy} style={{ width: '100%', marginTop: 8 }}>
            {busy ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  )
}
