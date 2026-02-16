import { SummaryCard } from '../components/SummaryCard'
import { TrafficChart } from '../components/TrafficChart'
import { useOverview, useTimeline } from '../hooks/useDashboardData'

function toPercent(ratio: number) {
  return `${(ratio * 100).toFixed(1)}%`
}

export function DashboardPage() {
  const overview = useOverview('24h')
  const timeline = useTimeline('24h', '1h')

  const isLoading = overview.isLoading || timeline.isLoading
  const hasError = overview.isError || timeline.isError

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

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-white">Traffic Overview</h2>
        <p className="mt-1 text-sm text-slate-400">Live gateway metrics for the last 24 hours.</p>
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
