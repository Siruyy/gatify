const STORAGE_KEYS = ['gatify.adminToken', 'adminToken'] as const

export function getRuntimeAdminToken(): string | undefined {
  if (typeof window === 'undefined') {
    return undefined
  }

  for (const key of STORAGE_KEYS) {
    const fromSession = window.sessionStorage.getItem(key)
    if (fromSession && fromSession.trim() !== '') {
      return fromSession.trim()
    }

    const fromLocal = window.localStorage.getItem(key)
    if (fromLocal && fromLocal.trim() !== '') {
      return fromLocal.trim()
    }
  }

  return undefined
}
