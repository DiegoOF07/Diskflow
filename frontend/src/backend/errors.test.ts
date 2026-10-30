import { describe, expect, it } from 'vitest'
import { BackendError, errorCode, errorMessage, toBackendError } from './errors'

describe('toBackendError', () => {
  it('keeps the code and detail Go sends, as JSON inside the Error the runtime makes', () => {
    const sent = new Error(
      JSON.stringify({ code: 'scan_running', detail: 'engine: a scan is already running' }),
    )
    const e = toBackendError(sent)
    expect(e).toBeInstanceOf(BackendError)
    expect(e.code).toBe('scan_running')
    expect(e.detail).toBe('engine: a scan is already running')
  })

  it('turns a bare string (a Wails error outside our methods) into a generic error', () => {
    const e = toBackendError('error parsing arguments')
    expect(e.code).toBe('error')
    expect(e.detail).toBe('error parsing arguments')
  })

  it('treats a code this version does not know as a generic error', () => {
    const e = toBackendError(new Error('{"code":"disk_on_fire","detail":"x"}'))
    expect(e.code).toBe('error')
    expect(e.detail).toBe('x')
  })

  it('keeps text that only looks like JSON', () => {
    expect(toBackendError(new Error('{not json')).detail).toBe('{not json')
  })

  it('returns a BackendError as is', () => {
    const e = new BackendError('not_found', 'gone')
    expect(toBackendError(e)).toBe(e)
  })
})

describe('errorMessage', () => {
  it('has a Spanish message for each code', () => {
    expect(errorMessage(new BackendError('scan_running'))).toMatch(/análisis en curso/)
    expect(errorMessage(new BackendError('root_unavailable'))).toMatch(/unidad/)
  })

  it('shows the detail of a generic error', () => {
    expect(errorMessage(new BackendError('error', 'boom'))).toBe('boom')
    expect(errorMessage(new Error('plain'))).toBe('plain')
  })
})

it('errorCode reads the code of anything thrown', () => {
  expect(errorCode(new BackendError('no_scan'))).toBe('no_scan')
  expect(errorCode('text')).toBe('error')
})
