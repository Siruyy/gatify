import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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

export type RulePayload = {
  name: string
  pattern: string
  methods: string[]
  priority: number
  limit: number
  window_seconds: number
  identify_by: 'ip' | 'header'
  header_name?: string
  enabled?: boolean
}

type UpdateRuleInput = {
  id: string
  payload: RulePayload
}

const RULES_QUERY_KEY = ['rules-list'] as const

export function useOverview(window = '24h') {
  return useQuery<Overview | null>({
    queryKey: ['stats-overview', window],
    queryFn: async () => {
      const payload = await apiRequest<ApiEnvelope<Overview>>(`/api/stats/overview?window=${encodeURIComponent(window)}`, {
        authToken: getRuntimeAdminToken({ useLegacyStorage: false }),
      })
      if (!payload) {
        return null
      }
      return payload.data
    },
  })
}

export function useTimeline(window = '24h', bucket = '1h') {
  return useQuery<TrafficPoint[] | null>({
    queryKey: ['stats-timeline', window, bucket],
    queryFn: async () => {
      const payload = await apiRequest<ApiEnvelope<TrafficPoint[]>>(
        `/api/stats/timeline?window=${encodeURIComponent(window)}&bucket=${encodeURIComponent(bucket)}`,
        {
          authToken: getRuntimeAdminToken({ useLegacyStorage: false }),
        },
      )
      if (!payload) {
        return null
      }
      return payload.data
    },
  })
}

export function useRules() {
  return useQuery<Rule[] | null>({
    queryKey: RULES_QUERY_KEY,
    queryFn: async () => {
      const payload = await apiRequest<ApiEnvelope<Rule[]>>('/api/rules', {
        authToken: getRuntimeAdminToken({ useLegacyStorage: false }),
      })
      if (!payload) {
        return null
      }
      return payload.data
    },
  })
}

export function useCreateRule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (payload: RulePayload) => {
      const response = await apiRequest<ApiEnvelope<Rule>>('/api/rules', {
        method: 'POST',
        authToken: getRuntimeAdminToken({ useLegacyStorage: false }),
        body: payload,
      })

      if (!response) {
        return null
      }

      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: RULES_QUERY_KEY })
    },
  })
}

export function useUpdateRule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, payload }: UpdateRuleInput) => {
      const response = await apiRequest<ApiEnvelope<Rule>>(`/api/rules/${encodeURIComponent(id)}`, {
        method: 'PUT',
        authToken: getRuntimeAdminToken({ useLegacyStorage: false }),
        body: payload,
      })

      if (!response) {
        return null
      }

      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: RULES_QUERY_KEY })
    },
  })
}

export function useDeleteRule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      await apiRequest<null>(`/api/rules/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        authToken: getRuntimeAdminToken({ useLegacyStorage: false }),
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: RULES_QUERY_KEY })
    },
  })
}
