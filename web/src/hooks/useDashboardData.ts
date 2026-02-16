import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../lib/api'
import type { ApiEnvelope } from '../lib/api'
import type { TrafficPoint } from '../components/TrafficChart'
import { getRuntimeAdminToken } from '../lib/auth'

export type Overview = {
  window_seconds: number
  total_requests: number
  allowed_requests: number
  blocked_requests: number
  unique_clients: number
  block_rate: number
}

export type Rule = {
  id: string
  name: string
  pattern: string
  methods: string[]
  priority: number
  limit: number
  window_seconds: number
  identify_by: string
  header_name?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export function useOverview(window = '24h') {
  return useQuery({
    queryKey: ['stats-overview', window],
    queryFn: async () => {
      const payload = await apiRequest<ApiEnvelope<Overview>>(`/api/stats/overview?window=${encodeURIComponent(window)}`, {
        authToken: getRuntimeAdminToken(),
      })
      return payload.data
    },
  })
}

export function useTimeline(window = '24h', bucket = '1h') {
  return useQuery({
    queryKey: ['stats-timeline', window, bucket],
    queryFn: async () => {
      const payload = await apiRequest<ApiEnvelope<TrafficPoint[]>>(
        `/api/stats/timeline?window=${encodeURIComponent(window)}&bucket=${encodeURIComponent(bucket)}`,
        {
          authToken: getRuntimeAdminToken(),
        },
      )
      return payload.data
    },
  })
}

export function useRules() {
  return useQuery({
    queryKey: ['rules-list'],
    queryFn: async () => {
      const payload = await apiRequest<ApiEnvelope<Rule[]>>('/api/rules', {
        authToken: getRuntimeAdminToken(),
      })
      return payload.data
    },
  })
}
