export const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:3000'

type RequestOptions = {
  method?: string
  body?: unknown
  authToken?: string
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers({
    'Content-Type': 'application/json',
  })

  if (options.authToken) {
    headers.set('Authorization', `Bearer ${options.authToken}`)
  }

  const response = await fetch(`${apiBaseURL}${path}`, {
    method: options.method ?? 'GET',
    headers,
    credentials: 'include',
    body: options.body ? JSON.stringify(options.body) : undefined,
  })

  if (!response.ok) {
    const errorText = await response.text()
    throw new Error(errorText || `Request failed with status ${response.status}`)
  }

  return (await response.json()) as T
}

export type ApiEnvelope<T> = {
  data: T
}
