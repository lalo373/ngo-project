const inr = new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR' })

/** Format a number as Indian Rupees, e.g. ₹1,234.50 */
export function money(amount: number): string {
  return inr.format(amount)
}

/** Format ISO date "2026-11-17" as "17 Nov 2026". */
export function formatDate(iso: string): string {
  const d = new Date(iso + 'T00:00:00')
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString('en-IN', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  })
}

/** Local date as "YYYY-MM-DD". */
export function todayIso(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** Local month as "YYYY-MM". */
export function currentMonth(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`
}
