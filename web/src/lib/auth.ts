type AdminTokenGetter = () => string | undefined

export type RuntimeAdminTokenOptions = {
  // Legacy opt-in only. Persisting admin credentials in browser storage is less secure.
  useLegacyStorage?: boolean
}

const LEGACY_STORAGE_KEYS = ['gatify.adminToken', 'adminToken'] as const

let inMemoryAdminToken: string | undefined
let runtimeAdminTokenGetter: AdminTokenGetter | undefined

export function setRuntimeAdminToken(token?: string): void {
  const trimmed = token?.trim()
  inMemoryAdminToken = trimmed ? trimmed : undefined
}

export function setRuntimeAdminTokenGetter(getter?: AdminTokenGetter): void {
  runtimeAdminTokenGetter = getter
}

export function getRuntimeAdminToken(options: RuntimeAdminTokenOptions = {}): string | undefined {
  const fromGetter = runtimeAdminTokenGetter?.()?.trim()
  if (fromGetter) {
    return fromGetter
  }

  if (inMemoryAdminToken) {
    return inMemoryAdminToken
  }

  if (!options.useLegacyStorage) {
    return undefined
  }

  return getLegacyStorageToken()
}

function getLegacyStorageToken(): string | undefined {
  if (typeof window === 'undefined') {
    return undefined
  }

  for (const key of LEGACY_STORAGE_KEYS) {
    const fromSession = window.sessionStorage.getItem(key)?.trim()
    if (fromSession) {
      return fromSession
    }

    const fromLocal = window.localStorage.getItem(key)?.trim()
    if (fromLocal) {
      return fromLocal
    }
  }

  return undefined
}
