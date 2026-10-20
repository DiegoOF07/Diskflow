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
