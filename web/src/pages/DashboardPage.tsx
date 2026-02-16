import { useEffect, useMemo, useRef, useState } from 'react'
import { SummaryCard } from '../components/SummaryCard'
import { TrafficChart } from '../components/TrafficChart'
import { useOverview, useTimeline } from '../hooks/useDashboardData'
import { apiBaseURL } from '../lib/api'
import { getRuntimeAdminToken } from '../lib/auth'

type RangeOption = {
  label: string
  window: string
  bucket: string
  description: string
}

const RANGE_OPTIONS: RangeOption[] = [
  {
    label: '1H',
    window: '1h',
    bucket: '1m',
    description: 'Live gateway metrics for the last hour.',
  },
  {
    label: '24H',
    window: '24h',
    bucket: '15m',
    description: 'Live gateway metrics for the last 24 hours.',
  },
  {
    label: '7D',
    window: '7d',
    bucket: '6h',
    description: 'Live gateway metrics for the last 7 days.',
  },
]

const LIVE_REFRESH_MS = 10_000

function toPercent(ratio: number) {
  return `${(ratio * 100).toFixed(1)}%`
}

function buildStatsStreamURL(token: string): string {
  const url = new URL(apiBaseURL)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.pathname = '/api/stats/stream'
  url.searchParams.set('token', token)
  return url.toString()
}

export function DashboardPage() {
  const [selectedWindow, setSelectedWindow] = useState('24h')
  const [liveEnabled, setLiveEnabled] = useState(true)
  const runtimeToken = getRuntimeAdminToken({ useLegacyStorage: false })

  const selectedRange = useMemo(() => {
    return RANGE_OPTIONS.find((option) => option.window === selectedWindow) ?? RANGE_OPTIONS[1]
  }, [selectedWindow])

  const refetchInterval = liveEnabled ? LIVE_REFRESH_MS : false

  const overview = useOverview(selectedRange.window, { refetchInterval })
  const timeline = useTimeline(selectedRange.window, selectedRange.bucket, { refetchInterval })
  const { refetch: refetchOverview } = overview
  const { refetch: refetchTimeline } = timeline

  const [streamStatus, setStreamStatus] = useState<'idle' | 'connecting' | 'connected' | 'error'>('idle')
  const lastRefreshAtRef = useRef(0)

  const isLoading = overview.isLoading || timeline.isLoading
  const isRefreshing = overview.isFetching || timeline.isFetching
  const hasError = overview.isError || timeline.isError

  const lastUpdated = useMemo(() => {
    const latest = Math.max(overview.dataUpdatedAt, timeline.dataUpdatedAt)
    if (!latest) {
      return null
    }

    return new Date(latest).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }, [overview.dataUpdatedAt, timeline.dataUpdatedAt])

  const handleManualRefresh = () => {
    void Promise.all([overview.refetch(), timeline.refetch()])
  }

  useEffect(() => {
    if (!liveEnabled || !runtimeToken) {
      return
    }

    let active = true
    const ws = new WebSocket(buildStatsStreamURL(runtimeToken))

    ws.onopen = () => {
      if (!active) {
        return
      }
      setStreamStatus('connected')
    }

    ws.onmessage = () => {
      if (!active) {
        return
      }

      const now = Date.now()
      if (now - lastRefreshAtRef.current < 2_000) {
        return
      }

      lastRefreshAtRef.current = now
      void refetchOverview()
      void refetchTimeline()
    }

    ws.onerror = () => {
      if (!active) {
        return
      }
      setStreamStatus('error')
    }

    ws.onclose = () => {
      if (!active) {
        return
      }
      setStreamStatus('idle')
    }

    return () => {
      active = false
      ws.close()
    }
  }, [liveEnabled, refetchOverview, refetchTimeline, runtimeToken])

  if (isLoading) {
    return <p className="text-slate-300">Loading dashboard data...</p>
  }

  if (hasError || !overview.data || !timeline.data) {
    return (
      <div className="rounded-xl border border-red-500/30 bg-red-950/30 p-4 text-red-200">
        Failed to load dashboard data. Ensure `VITE_API_BASE_URL` and runtime admin auth are configured.
      </div>
    )
  }

  const streamStatusLabel = !liveEnabled || !runtimeToken
    ? 'idle'
    : streamStatus === 'idle'
      ? 'connecting…'
      : streamStatus

  return (
    <section className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-white">Traffic Overview</h2>
          <p className="mt-1 text-sm text-slate-400">{selectedRange.description}</p>
          <p className="mt-1 text-xs text-slate-500">
            Live stream: {streamStatusLabel}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="rounded-xl border border-slate-700 bg-slate-900 p-1">
            {RANGE_OPTIONS.map((option) => {
              const isActive = option.window === selectedRange.window
              return (
                <button
                  key={option.window}
                  type="button"
                  onClick={() => setSelectedWindow(option.window)}
                  className={[
                    'rounded-lg px-3 py-1.5 text-xs font-medium transition',
                    isActive ? 'bg-cyan-500/20 text-cyan-300' : 'text-slate-300 hover:bg-slate-800 hover:text-white',
                  ].join(' ')}
                >
                  {option.label}
                </button>
              )
            })}
          </div>

          <button
            type="button"
            onClick={() => setLiveEnabled((prev) => !prev)}
            className={[
              'rounded-lg border px-3 py-1.5 text-xs font-medium transition',
              liveEnabled
                ? 'border-emerald-400/40 bg-emerald-500/10 text-emerald-300 hover:bg-emerald-500/20'
                : 'border-slate-700 text-slate-300 hover:bg-slate-800 hover:text-white',
            ].join(' ')}
          >
            {liveEnabled ? 'Live: On' : 'Live: Off'}
          </button>

          <button
            type="button"
            onClick={handleManualRefresh}
            className="rounded-lg border border-slate-700 px-3 py-1.5 text-xs font-medium text-slate-300 transition hover:bg-slate-800 hover:text-white"
          >
            Refresh
          </button>
        </div>
      </div>

      <div className="text-xs text-slate-400">
        {isRefreshing ? 'Updating metrics…' : `Last updated at ${lastUpdated ?? '—'}`}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <SummaryCard title="Total Requests" value={overview.data.total_requests.toLocaleString()} />
        <SummaryCard title="Allowed" value={overview.data.allowed_requests.toLocaleString()} />
        <SummaryCard title="Blocked" value={overview.data.blocked_requests.toLocaleString()} />
        <SummaryCard
          title="Block Rate"
          value={toPercent(overview.data.block_rate)}
          subtitle={`${overview.data.unique_clients.toLocaleString()} unique clients`}
        />
      </div>

      <TrafficChart data={timeline.data} />
    </section>
  )
}
