const oneDecimal = new Intl.NumberFormat('es', { minimumFractionDigits: 1, maximumFractionDigits: 1 })
const integer = new Intl.NumberFormat('es')

/** Formats a byte count with binary units: 1536 -> "1,5 KiB". */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  let value = n / 1024
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${oneDecimal.format(value)} ${units[i]}`
}

/** part as a percentage of total, or "—" when total is 0. */
export function formatPercent(part: number, total: number): string {
  return total > 0 ? `${oneDecimal.format((100 * part) / total)} %` : '—'
}

export function formatCount(n: number): string {
  return integer.format(n)
}

const relative = new Intl.RelativeTimeFormat('es', { numeric: 'auto' })

/** Describes how long ago a unix time was: "hace 3 horas", "ayer". */
export function formatAgo(unixSeconds: number, nowSeconds = Date.now() / 1000): string {
  const s = Math.max(nowSeconds - unixSeconds, 0)
  if (s < 60) return 'hace un momento'
  if (s < 3600) return relative.format(-Math.floor(s / 60), 'minute')
  if (s < 86400) return relative.format(-Math.floor(s / 3600), 'hour')
  return relative.format(-Math.floor(s / 86400), 'day')
}

// Spelled out because dateStyle 'short' gives a two-digit year in Spanish.
const dateTime = new Intl.DateTimeFormat('es', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

/** Formats a unix time as a local date and time, or "—" when it is unknown (0). */
export function formatDate(unixSeconds: number): string {
  return unixSeconds ? dateTime.format(unixSeconds * 1000) : '—'
}
