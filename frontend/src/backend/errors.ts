/** Codes of internal/desktop/errors.go. */
export type ErrorCode =
  'no_scan' | 'root_unavailable' | 'scan_running' | 'db_unavailable' | 'access_denied' | 'not_found' | 'error' // no specific message; the detail is shown

// The only place with the text the user reads for each code.
const MESSAGES: Record<Exclude<ErrorCode, 'error'>, string> = {
  no_scan: 'Esta carpeta aún no tiene un análisis completo.',
  root_unavailable:
    'No se encuentra la carpeta: puede que se haya movido o borrado, o que su unidad esté desconectada.',
  scan_running: 'Ya hay un análisis en curso. Espera a que termine o cancélalo antes de empezar otro.',
  db_unavailable:
    'No se pudo abrir el índice de DiskFlow. Cierra la app y vuelve a abrirla; si sigue fallando, revisa el espacio libre del disco.',
  access_denied: 'Windows no permite leer esa ubicación.',
  not_found: 'Ya no existe: cambió después del último análisis. Analiza de nuevo para actualizar.',
}

/** What every Backend method rejects with. */
export class BackendError extends Error {
  constructor(
    readonly code: ErrorCode,
    readonly detail = '',
  ) {
    super(detail || code)
    this.name = 'BackendError'
  }
}

const known = (code: unknown): code is ErrorCode =>
  typeof code === 'string' && (code === 'error' || code in MESSAGES)

/**
 * Turns what a Wails call rejects with into a BackendError. Go sends
 * {code, detail} as JSON text, which the runtime wraps in an Error; errors
 * that don't come from our methods (invalid arguments) are plain text.
 */
export function toBackendError(e: unknown): BackendError {
  if (e instanceof BackendError) return e
  const text = e instanceof Error ? e.message : String(e)
  const sent = parseSent(text)
  if (!sent) return new BackendError('error', text)
  return new BackendError(known(sent.code) ? sent.code : 'error', String(sent.detail ?? ''))
}

function parseSent(text: string): { code: unknown; detail?: unknown } | null {
  if (!text.startsWith('{')) return null
  try {
    const v: unknown = JSON.parse(text)
    return typeof v === 'object' && v !== null && 'code' in v ? (v as { code: unknown }) : null
  } catch {
    return null
  }
}

/** The message for the user. */
export function errorMessage(e: unknown): string {
  const err = toBackendError(e)
  return err.code === 'error' ? err.detail || 'Error inesperado.' : MESSAGES[err.code]
}

export const errorCode = (e: unknown): ErrorCode => toBackendError(e).code
