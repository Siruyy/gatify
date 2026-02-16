export const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:3000'

type RequestOptions = {
  method?: string
  body?: unknown
  authToken?: string
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers()
  const method = (options.method ?? 'GET').toUpperCase()
  const methodImpliesBody = method === 'POST' || method === 'PUT' || method === 'PATCH'

  if (options.authToken) {
    headers.set('Authorization', `Bearer ${options.authToken}`)
  }

  if (options.body || methodImpliesBody) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(`${apiBaseURL}${path}`, {
    method,
    headers,
    credentials: 'include',
    body: options.body ? JSON.stringify(options.body) : undefined,
  })

  if (!response.ok) {
    const errorText = await response.text()
    throw new Error(errorText || `Request failed with status ${response.status}`)
  }

  const text = await response.text()
  if (response.status === 204 || response.status === 205 || text.trim() === '') {
    return null as unknown as T
  }

  return JSON.parse(text) as T
}

export type ApiEnvelope<T> = {
  data: T
}
