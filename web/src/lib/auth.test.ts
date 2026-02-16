import { afterEach, describe, expect, it } from 'vitest'
import {
  getRuntimeAdminToken,
  setRuntimeAdminToken,
  setRuntimeAdminTokenGetter,
} from './auth'

describe('auth', () => {
  afterEach(() => {
    // Reset state between tests
    setRuntimeAdminToken(undefined)
    setRuntimeAdminTokenGetter(undefined)
  })

  describe('setRuntimeAdminToken / getRuntimeAdminToken', () => {
    it('returns undefined when no token is set', () => {
      expect(getRuntimeAdminToken()).toBeUndefined()
    })

    it('returns the token that was set', () => {
      setRuntimeAdminToken('my-secret')
      expect(getRuntimeAdminToken()).toBe('my-secret')
    })

    it('trims whitespace from token', () => {
      setRuntimeAdminToken('  spaced  ')
      expect(getRuntimeAdminToken()).toBe('spaced')
    })

    it('treats empty/whitespace-only token as undefined', () => {
      setRuntimeAdminToken('   ')
      expect(getRuntimeAdminToken()).toBeUndefined()
    })

    it('can clear a previously set token', () => {
      setRuntimeAdminToken('my-secret')
      setRuntimeAdminToken(undefined)
      expect(getRuntimeAdminToken()).toBeUndefined()
    })
  })

  describe('setRuntimeAdminTokenGetter', () => {
    it('uses getter over in-memory token', () => {
      setRuntimeAdminToken('in-memory')
      setRuntimeAdminTokenGetter(() => 'from-getter')
      expect(getRuntimeAdminToken()).toBe('from-getter')
    })

    it('falls back to in-memory when getter returns undefined', () => {
      setRuntimeAdminToken('in-memory')
      setRuntimeAdminTokenGetter(() => undefined)
      expect(getRuntimeAdminToken()).toBe('in-memory')
    })

    it('falls back to in-memory when getter returns empty string', () => {
      setRuntimeAdminToken('in-memory')
      setRuntimeAdminTokenGetter(() => '   ')
      expect(getRuntimeAdminToken()).toBe('in-memory')
    })
  })
})
