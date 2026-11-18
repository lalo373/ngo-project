import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ROLE_LABELS, type CalendarResponse, type DailyPayout, type Me, type WeeklyPayouts } from '../api'
import { currentMonth, formatDate, money } from '../format'

type Tab = 'daily' | 'weekly' | 'calendar'

function Header({ me }: { me: Me }) {
  const navigate = useNavigate()

  async function logout() {
    try {
      await api('/logout', { method: 'POST' })
    } finally {
      navigate('/login', { replace: true })
    }
  }

  return (
    <header className="topbar">
      <div className="topbar-brand">Payroll</div>
      <div className="topbar-user">
        <div style={{ textAlign: 'right' }}>
          <div className="topbar-name">{me.name}</div>
          <div className="topbar-role">{ROLE_LABELS[me.role] ?? me.role}</div>
        </div>
        <button className="btn btn-secondary btn-small" onClick={logout}>
          Logout
        </button>
      </div>
    </header>
  )
}

function DailyTab({ me }: { me: Me }) {
  const [data, setData] = useState<DailyPayout | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    api<DailyPayout>('/dailyPayout')
      .then((d) => !cancelled && setData(d))
      .catch((e) => !cancelled && setError(e instanceof Error ? e.message : 'Failed to load'))
    return () => {
      cancelled = true
    }
  }, [])

  const showPercent = me.role === 'supervisor' || me.role === 'district_coordinator'
  const metricLabel = me.role === 'executive' ? 'Target' : 'Eligibility'

  if (error) return <div className="error">{error}</div>
  if (!data) return <div className="loading">Loading…</div>

  return (
    <div className="card hero">
      <div className="hero-date">Today · {formatDate(data.date)}</div>
      <div>
        <span className={`badge ${data.eligible ? 'badge-eligible' : 'badge-not-eligible'}`}>
          {data.eligible ? 'Eligible' : 'Not eligible'}
        </span>
      </div>
      <div className={`hero-amount ${data.eligible ? '' : 'muted'}`}>{money(data.amount)}</div>
      <div className="hero-stats">
        <div>
          <div className="stat-label">{metricLabel}</div>
          <div className="stat-value">
            {showPercent ? `${data.metric.toFixed(1)}%` : data.metric}
          </div>
        </div>
        <div>
          <div className="stat-label">This week</div>
          <div className="stat-value">{money(data.weekly_total)}</div>
        </div>
      </div>
    </div>
  )
}

function WeeklyTab() {
  const [data, setData] = useState<WeeklyPayouts | null>(null)
  const [error, setError] = useState('')
  const [open, setOpen] = useState<Record<number, boolean>>({ 0: true })

  useEffect(() => {
    let cancelled = false
    api<WeeklyPayouts>('/weeklyPayouts')
      .then((d) => !cancelled && setData(d))
      .catch((e) => !cancelled && setError(e instanceof Error ? e.message : 'Failed to load'))
    return () => {
      cancelled = true
    }
  }, [])

  if (error) return <div className="error">{error}</div>
  if (!data) return <div className="loading">Loading…</div>
  if (data.weeks.length === 0) return <div className="card muted">No payouts yet.</div>

  const rows: React.ReactNode[] = []
  data.weeks.forEach((w, i) => {
    const isOpen = !!open[i]
    rows.push(
      <tr key={`w${i}`} className="row-head" onClick={() => setOpen((o) => ({ ...o, [i]: !o[i] }))}>
        <td>
          <span className={`week-chevron ${isOpen ? 'open' : ''}`}>▸</span>
          {formatDate(w.week_start)} – {formatDate(w.week_end)}
        </td>
        <td className="num">{money(w.total)}</td>
      </tr>,
    )
    if (isOpen) {
      rows.push(
        <tr key={`d${i}`} className="nested-row">
          <td colSpan={2}>
            <table>
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Eligible</th>
                  <th className="num">Payout</th>
                </tr>
              </thead>
              <tbody>
                {w.days.map((d) => (
                  <tr key={d.date}>
                    <td>{formatDate(d.date)}</td>
                    <td>{d.eligible ? 'Yes' : 'No'}</td>
                    <td className="num">{d.amount > 0 ? money(d.amount) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </td>
        </tr>,
      )
    }
  })

  return (
    <div className="card">
      <h2>Weekly payouts</h2>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Week (Mon – Sun)</th>
              <th className="num">Total</th>
            </tr>
          </thead>
          <tbody>{rows}</tbody>
        </table>
      </div>
    </div>
  )
}

function CalendarTab() {
  const [month, setMonth] = useState(currentMonth())
  const [data, setData] = useState<CalendarResponse | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api<CalendarResponse>(`/calendar?month=${encodeURIComponent(month)}`)
      .then((d) => {
        if (!cancelled) {
          setData(d)
          setLoading(false)
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : 'Failed to load')
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [month])

  // Month options: the current month and the five months before it.
  const options: string[] = []
  const now = new Date()
  for (let i = 0; i < 6; i++) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
    const pad = (n: number) => String(n).padStart(2, '0')
    options.push(`${d.getFullYear()}-${pad(d.getMonth() + 1)}`)
  }

  return (
    <div className="card">
      <div className="card-title-row">
        <h2>Calendar</h2>
        <div className="inline-controls">
          <label className="field" style={{ marginBottom: 0 }}>
            <span className="muted" style={{ fontSize: 13 }}>Month</span>
            <select value={month} onChange={(e) => setMonth(e.target.value)}>
              {options.map((m) => (
                <option key={m} value={m}>
                  {formatDate(m + '-01').replace(/^\d+\s/, '')}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>
      {error && <div className="error">{error}</div>}
      {loading ? (
        <div className="loading">Loading…</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Date</th>
                <th className="num">Payout</th>
              </tr>
            </thead>
            <tbody>
              {data?.days.map((d) => (
                <tr key={d.date}>
                  <td>{formatDate(d.date)}</td>
                  <td className="num">{d.amount > 0 ? money(d.amount) : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

export default function Dashboard() {
  const navigate = useNavigate()
  const [me, setMe] = useState<Me | null>(null)
  const [tab, setTab] = useState<Tab>('daily')

  useEffect(() => {
    let cancelled = false
    api<Me>('/me')
      .then((m) => {
        if (!cancelled) {
          if (m.role === 'admin') {
            navigate('/admin', { replace: true })
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

  if (!me) return <div className="loading">Loading…</div>

  return (
    <>
      <Header me={me} />
      <div className="container">
        <div className="tabs">
          <button className={`tab ${tab === 'daily' ? 'active' : ''}`} onClick={() => setTab('daily')}>
            Daily Payout
          </button>
          <button className={`tab ${tab === 'weekly' ? 'active' : ''}`} onClick={() => setTab('weekly')}>
            Weekly Payouts
          </button>
          <button className={`tab ${tab === 'calendar' ? 'active' : ''}`} onClick={() => setTab('calendar')}>
            Calendar
          </button>
        </div>
        {tab === 'daily' && <DailyTab me={me} />}
        {tab === 'weekly' && <WeeklyTab />}
        {tab === 'calendar' && <CalendarTab />}
      </div>
    </>
  )
}
