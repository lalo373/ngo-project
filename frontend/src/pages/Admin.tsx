import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  api,
  downloadCsv,
  MEMBER_ROLES,
  ROLE_LABELS,
  type Member,
  type Me,
  type Role,
} from '../api'
import { money, todayIso } from '../format'

function MemberEditModal({
  role,
  member,
  onClose,
  onSaved,
}: {
  role: Role
  member: Member
  onClose: () => void
  onSaved: () => void
}) {
  const [name, setName] = useState(member.name)
  const [phone, setPhone] = useState(member.phone)
  const [area, setArea] = useState(member.area)
  const [parentId, setParentId] = useState(String(member.parent_id))
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function save() {
    setBusy(true)
    setError('')
    try {
      await api(`/admin/members/${role}/${member.id}`, {
        method: 'PUT',
        body: {
          name,
          phone,
          area,
          parent_id: Number(parentId) || 0,
        },
      })
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
      setBusy(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>
          Edit {ROLE_LABELS[role]} #{member.id}
        </h3>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void save()
          }}
        >
          <label className="field">
            <span>Name</span>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </label>
          <label className="field">
            <span>Phone</span>
            <input value={phone} onChange={(e) => setPhone(e.target.value)} />
          </label>
          <label className="field">
            <span>Area</span>
            <input value={area} onChange={(e) => setArea(e.target.value)} />
          </label>
          <label className="field">
            <span>Parent ID</span>
            <input
              type="number"
              min={0}
              value={parentId}
              onChange={(e) => setParentId(e.target.value)}
            />
          </label>
          {error && <div className="error">{error}</div>}
          <div className="modal-actions">
            <button type="button" className="btn btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function PasswordModal({
  role,
  member,
  onClose,
}: {
  role: Role
  member: Member
  onClose: () => void
}) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const [busy, setBusy] = useState(false)

  async function save() {
    setBusy(true)
    setError('')
    try {
      await api(`/admin/members/${role}/${member.id}/password`, {
        method: 'PUT',
        body: { password },
      })
      setDone(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Reset failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>
          Reset password · {ROLE_LABELS[role]} #{member.id} ({member.name})
        </h3>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            void save()
          }}
        >
          <label className="field">
            <span>New password</span>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              required
              minLength={4}
            />
          </label>
          {error && <div className="error">{error}</div>}
          {done && <div className="success">Password updated.</div>}
          <div className="modal-actions">
            <button type="button" className="btn btn-secondary" onClick={onClose}>
              Close
            </button>
            <button type="submit" className="btn" disabled={busy || done}>
              {busy ? 'Saving…' : 'Reset password'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function MembersCard() {
  const [role, setRole] = useState<Role>('executive')
  const [members, setMembers] = useState<Member[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<Member | null>(null)
  const [pwTarget, setPwTarget] = useState<Member | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<Member | null>(null)

  // Add-member form
  const [showAdd, setShowAdd] = useState(false)
  const [newName, setNewName] = useState('')
  const [newPhone, setNewPhone] = useState('')
  const [newArea, setNewArea] = useState('')
  const [newParentId, setNewParentId] = useState('')
  const [newPassword, setNewPassword] = useState('')

  const load = useCallback(
    async (r: Role) => {
      setLoading(true)
      setError('')
      try {
        const data = await api<{ members: Member[] }>(
          `/admin/members?role=${encodeURIComponent(r)}`,
        )
        setMembers(data.members)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load members')
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  useEffect(() => {
    void load(role)
  }, [role, load])

  async function addMember(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setNotice('')
    try {
      const res = await api<{ id: number }>('/admin/members', {
        method: 'POST',
        body: {
          role,
          name: newName,
          phone: newPhone,
          area: newArea,
          parent_id: Number(newParentId) || 0,
          password: newPassword,
        },
      })
      setNotice(`Added ${ROLE_LABELS[role]} #${res.id}.`)
      setNewName('')
      setNewPhone('')
      setNewArea('')
      setNewParentId('')
      setNewPassword('')
      setShowAdd(false)
      void load(role)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add member')
    }
  }

  async function deleteMember(member: Member) {
    setError('')
    setNotice('')
    try {
      await api(`/admin/members/${role}/${member.id}`, { method: 'DELETE' })
      setNotice(`Deleted ${ROLE_LABELS[role]} #${member.id}.`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete member')
    }
    setConfirmDelete(null)
    void load(role)
  }

  return (
    <div className="card">
      <div className="card-title-row">
        <h2>Members</h2>
        <div className="inline-controls">
          <select value={role} onChange={(e) => setRole(e.target.value as Role)}>
            {MEMBER_ROLES.map((r) => (
              <option key={r} value={r}>
                {ROLE_LABELS[r]}
              </option>
            ))}
          </select>
          <button
            className="btn btn-secondary btn-small"
            onClick={() => setShowAdd((s) => !s)}
          >
            {showAdd ? 'Close form' : 'Add member'}
          </button>
        </div>
      </div>

      {error && <div className="error">{error}</div>}
      {notice && <div className="success">{notice}</div>}

      {showAdd && (
        <form
          onSubmit={addMember}
          style={{ marginBottom: 16, padding: 16, border: '1px solid var(--border)', borderRadius: 10 }}
        >
          <div className="form-row" style={{ marginBottom: 12 }}>
            <label className="field">
              <span>Name</span>
              <input value={newName} onChange={(e) => setNewName(e.target.value)} required />
            </label>
            <label className="field">
              <span>Phone</span>
              <input value={newPhone} onChange={(e) => setNewPhone(e.target.value)} />
            </label>
          </div>
          <div className="form-row" style={{ marginBottom: 12 }}>
            <label className="field">
              <span>Area</span>
              <input value={newArea} onChange={(e) => setNewArea(e.target.value)} />
            </label>
            <label className="field">
              <span>Parent ID</span>
              <input
                type="number"
                min={0}
                value={newParentId}
                onChange={(e) => setNewParentId(e.target.value)}
              />
            </label>
          </div>
          <div className="form-row" style={{ marginBottom: 12 }}>
            <label className="field" style={{ flex: 2, minWidth: 220 }}>
              <span>Password</span>
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                minLength={4}
                autoComplete="new-password"
              />
            </label>
            <div>
              <button type="submit" className="btn">
                Create {ROLE_LABELS[role]}
              </button>
            </div>
          </div>
        </form>
      )}

      {loading ? (
        <div className="loading">Loading…</div>
      ) : members.length === 0 ? (
        <div className="muted">No {ROLE_LABELS[role]}s found.</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Name</th>
                <th>Phone</th>
                <th>Area</th>
                <th>Parent ID</th>
                <th className="num">Actions</th>
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <tr key={m.id}>
                  <td>{m.id}</td>
                  <td>{m.name}</td>
                  <td>{m.phone || '—'}</td>
                  <td>{m.area || '—'}</td>
                  <td>{m.parent_id || '—'}</td>
                  <td className="num">
                    <div className="actions" style={{ justifyContent: 'flex-end' }}>
                      <button className="btn btn-secondary btn-small" onClick={() => setEditing(m)}>
                        Edit
                      </button>
                      <button
                        className="btn btn-secondary btn-small"
                        onClick={() => setPwTarget(m)}
                      >
                        Reset password
                      </button>
                      <button className="btn btn-danger btn-small" onClick={() => setConfirmDelete(m)}>
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {editing && (
        <MemberEditModal
          role={role}
          member={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setNotice(`Saved changes to #${editing.id}.`)
            void load(role)
          }}
        />
      )}
      {pwTarget && (
        <PasswordModal role={role} member={pwTarget} onClose={() => setPwTarget(null)} />
      )}
      {confirmDelete && (
        <div className="modal-overlay" onClick={() => setConfirmDelete(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>Delete {ROLE_LABELS[role]} #{confirmDelete.id}?</h3>
            <p className="muted" style={{ margin: '0 0 8px' }}>
              This will permanently remove <strong>{confirmDelete.name}</strong>. This action
              cannot be undone.
            </p>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setConfirmDelete(null)}>
                Cancel
              </button>
              <button className="btn btn-danger" onClick={() => void deleteMember(confirmDelete)}>
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function ReportsCard() {
  const [date, setDate] = useState(todayIso())
  const [status, setStatus] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function download() {
    setBusy(true)
    setError('')
    setStatus('')
    try {
      await downloadCsv(date)
      setStatus(`Report for ${date} downloaded.`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Download failed')
    } finally {
      setBusy(false)
    }
  }

  async function runPayout() {
    setBusy(true)
    setError('')
    setStatus('')
    try {
      const qs = date ? `?date=${encodeURIComponent(date)}` : ''
      const res = await api<{ computed: number }>(`/admin/runPayout${qs}`, { method: 'POST' })
      setStatus(`Payout computed for ${date}: ${money(res.computed)} paid out.`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Payout failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="card">
      <h2>Reports</h2>
      <div className="inline-controls">
        <input
          type="date"
          value={date}
          onChange={(e) => setDate(e.target.value)}
          style={{ width: 180 }}
        />
        <button className="btn" onClick={() => void download()} disabled={busy}>
          Download CSV
        </button>
        <button className="btn btn-secondary" onClick={() => void runPayout()} disabled={busy}>
          Run payout computation
        </button>
      </div>
      {error && <div className="error">{error}</div>}
      {status && <div className="success">{status}</div>}
    </div>
  )
}

export default function Admin() {
  const navigate = useNavigate()
  const [me, setMe] = useState<Me | null>(null)

  useEffect(() => {
    let cancelled = false
    api<Me>('/me')
      .then((m) => {
        if (!cancelled) {
          if (m.role !== 'admin') {
            navigate(m.role ? '/dashboard' : '/login', { replace: true })
          } else {
            setMe(m)
          }
        }
      })
      .catch(() => {
        // 401 — the API helper already redirected to /login.
      })
    return () => {
      cancelled = true
    }
  }, [navigate])

  async function logout() {
    try {
      await api('/logout', { method: 'POST' })
    } finally {
      navigate('/login', { replace: true })
    }
  }

  if (!me) return <div className="loading">Loading…</div>

  return (
    <>
      <header className="topbar">
        <div className="topbar-brand">Payroll · Admin</div>
        <div className="topbar-user">
          <div style={{ textAlign: 'right' }}>
            <div className="topbar-name">{me.name}</div>
            <div className="topbar-role">Admin</div>
          </div>
          <button className="btn btn-secondary btn-small" onClick={() => void logout()}>
            Logout
          </button>
        </div>
      </header>
      <div className="container-wide">
        <ReportsCard />
        <MembersCard />
      </div>
    </>
  )
}
