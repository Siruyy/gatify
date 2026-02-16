import { useMemo, useState } from 'react'
import { TrafficChart } from '../components/TrafficChart'
import { useTimeline, useTopBlocked } from '../hooks/useDashboardData'

type WindowOption = {
  label: string
  window: string
  bucket: string
}

const WINDOW_OPTIONS: WindowOption[] = [
  { label: '1H', window: '1h', bucket: '1m' },
  { label: '24H', window: '24h', bucket: '15m' },
  { label: '7D', window: '7d', bucket: '6h' },
]

function downloadCsv(filename: string, headers: string[], rows: string[][]) {
  const content = [headers.join(','), ...rows.map((row) => row.map(escapeCsv).join(','))].join('\n')
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)

  const anchor = document.createElement('a')
  anchor.href = url
  anchor.setAttribute('download', filename)
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)

  URL.revokeObjectURL(url)
}

function escapeCsv(value: string) {
  if (value.includes(',') || value.includes('"') || value.includes('\n')) {
    return `"${value.replaceAll('"', '""')}"`
  }
  return value
}

export function AnalyticsPage() {
  const [selectedWindow, setSelectedWindow] = useState('24h')

  const selectedRange = useMemo(() => {
    return WINDOW_OPTIONS.find((option) => option.window === selectedWindow) ?? WINDOW_OPTIONS[1]
  }, [selectedWindow])

  const timelineQuery = useTimeline(selectedRange.window, selectedRange.bucket)
  const topBlockedQuery = useTopBlocked(selectedRange.window, 15)

  const isLoading = timelineQuery.isLoading || topBlockedQuery.isLoading
  const hasError = timelineQuery.isError || topBlockedQuery.isError

  if (isLoading) {
    return <p className="text-slate-300">Loading analytics...</p>
  }

  if (hasError || !timelineQuery.data || !topBlockedQuery.data) {
    return (
      <div className="rounded-xl border border-red-500/30 bg-red-950/30 p-4 text-red-200">
        Failed to load analytics data. Ensure `VITE_API_BASE_URL` and runtime admin auth are configured.
      </div>
    )
  }

  const topBlocked = topBlockedQuery.data
  const timeline = timelineQuery.data

  const exportTopBlockedCsv = () => {
    const rows = topBlocked.map((item) => [item.client_id, String(item.blocked_count)])
    downloadCsv(`top-blocked-${selectedRange.window}.csv`, ['client_id', 'blocked_count'], rows)
  }

  const exportTimelineCsv = () => {
    const rows = timeline.map((item) => [
      item.bucket_start,
      String(item.allowed),
      String(item.blocked),
      String(item.total),
    ])
    downloadCsv(
      `timeline-${selectedRange.window}.csv`,
      ['bucket_start', 'allowed', 'blocked', 'total'],
      rows,
    )
  }

  return (
    <section className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-white">Analytics & Insights</h2>
          <p className="mt-1 text-sm text-slate-400">Detailed traffic and blocking insights by window.</p>
        </div>

        <div className="rounded-xl border border-slate-700 bg-slate-900 p-1">
          {WINDOW_OPTIONS.map((option) => {
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
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-900 shadow-lg shadow-slate-950/30 lg:col-span-2">
          <div className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
            <h3 className="text-sm font-medium text-slate-200">Top Blocked Clients</h3>
            <button
              type="button"
              onClick={exportTopBlockedCsv}
              className="rounded-lg border border-slate-700 px-3 py-1 text-xs text-slate-300 transition hover:bg-slate-800 hover:text-white"
            >
              Export CSV
            </button>
          </div>
          <table className="min-w-full divide-y divide-slate-800 text-sm">
            <thead className="bg-slate-900/80 text-left text-slate-300">
              <tr>
                <th scope="col" className="px-4 py-3 font-medium">Client ID</th>
                <th scope="col" className="px-4 py-3 font-medium">Blocked Requests</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800 text-slate-100">
              {topBlocked.length === 0 ? (
                <tr>
                  <td colSpan={2} className="px-4 py-8 text-center text-slate-400">
                    No blocked clients found for this window.
                  </td>
                </tr>
              ) : (
                topBlocked.map((item) => (
                  <tr key={item.client_id}>
                    <td className="px-4 py-3 font-mono text-xs text-slate-300">{item.client_id}</td>
                    <td className="px-4 py-3">{item.blocked_count.toLocaleString()}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="rounded-2xl border border-slate-800 bg-slate-900 p-4 shadow-lg shadow-slate-950/30">
          <h3 className="text-sm font-medium text-slate-200">Quick Stats</h3>
          <dl className="mt-4 space-y-3 text-sm">
            <div className="flex items-center justify-between">
              <dt className="text-slate-400">Blocked Clients</dt>
              <dd className="font-medium text-white">{topBlocked.length}</dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-slate-400">Total Blocked</dt>
              <dd className="font-medium text-white">
                {topBlocked.reduce((sum, item) => sum + item.blocked_count, 0).toLocaleString()}
              </dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-slate-400">Timeline Points</dt>
              <dd className="font-medium text-white">{timeline.length}</dd>
            </div>
          </dl>
        </div>
      </div>

      <div>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-medium text-slate-200">Request Timeline</h3>
          <button
            type="button"
            onClick={exportTimelineCsv}
            className="rounded-lg border border-slate-700 px-3 py-1 text-xs text-slate-300 transition hover:bg-slate-800 hover:text-white"
          >
            Export CSV
          </button>
        </div>
        <TrafficChart data={timeline} />
      </div>
    </section>
  )
}
