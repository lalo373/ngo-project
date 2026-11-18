export type Role =
  | 'executive'
  | 'supervisor'
  | 'district_coordinator'
  | 'regional_coordinator'
  | 'admin'

export interface Me {
  role: Role
  id: number
  name: string
}

export interface DailyPayout {
  date: string
  eligible: boolean
  amount: number
  metric: number
  weekly_total: number
}

export interface DayEntry {
  date: string
  eligible: boolean
  amount: number
  /** executives: daily target count; supervisors/DCs: eligible percent */
  metric: number
}

export interface Week {
  week_start: string
  week_end: string
  total: number
  days: DayEntry[]
}

export interface WeeklyPayouts {
  weeks: Week[]
}

export interface CalendarDay {
  date: string
  amount: number
}

export interface CalendarResponse {
  month: string
  days: CalendarDay[]
}

export interface Member {
  id: number
  name: string
  phone: string
  area: string
  parent_id: number
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

function redirectToLogin() {
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
}

async function parseError(res: Response): Promise<string> {
  try {
    const data = (await res.json()) as { error?: string }
    return data.error || `Request failed (${res.status})`
  } catch {
    return `Request failed (${res.status})`
  }
}

/**
 * Tiny JSON fetch wrapper. Same-origin fetch includes the `sid` cookie
 * automatically, so no credentials option is needed.
 * On any 401 (except when already on the login page) redirects to /login.
 */
export async function api<T>(
  path: string,
  options: { method?: string; body?: unknown } = {},
): Promise<T> {
  const res = await fetch(path, {
    method: options.method ?? 'GET',
    headers: options.body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  })

  if (!res.ok) {
    const message = await parseError(res)
    if (res.status === 401) redirectToLogin()
    throw new ApiError(res.status, message)
  }
  return (await res.json()) as T
}

/** Fetch a CSV attachment and trigger a browser download. */
export async function downloadCsv(date: string): Promise<void> {
  const res = await fetch(`/admin/dashboard/getDataByDate?date=${encodeURIComponent(date)}`)
  if (!res.ok) {
    const message = await parseError(res)
    if (res.status === 401) redirectToLogin()
    throw new ApiError(res.status, message)
  }
  const blob = await res.blob()
  const disposition = res.headers.get('Content-Disposition') || ''
  const match = /filename="?([^";]+)"?/.exec(disposition)
  const filename = match ? match[1] : `report_${date}.csv`
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export const ROLE_LABELS: Record<Role, string> = {
  executive: 'Executive',
  supervisor: 'Supervisor',
  district_coordinator: 'District Coordinator',
  regional_coordinator: 'Regional Coordinator',
  admin: 'Admin',
}

/** Member roles managed on the admin page (excludes admin itself). */
export const MEMBER_ROLES: Role[] = [
  'executive',
  'supervisor',
  'district_coordinator',
  'regional_coordinator',
]
