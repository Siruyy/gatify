import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

export type TrafficPoint = {
  bucket_start: string
  allowed: number
  blocked: number
  total: number
}

type TrafficChartProps = {
  data: TrafficPoint[]
}

function formatTimeLabel(raw: string) {
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return raw
  }

  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function tooltipLabelFormatter(label: unknown) {
  if (typeof label !== 'string') {
    return String(label)
  }

  return formatTimeLabel(label)
}

export function TrafficChart({ data }: TrafficChartProps) {
  return (
    <div className="h-80 w-full rounded-2xl border border-slate-800 bg-slate-900 p-4 shadow-lg shadow-slate-950/30">
      <p className="mb-4 text-sm font-medium text-slate-200">Traffic Timeline</p>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 8 }}>
          <CartesianGrid stroke="#334155" strokeDasharray="3 3" />
          <XAxis
            dataKey="bucket_start"
            tickFormatter={formatTimeLabel}
            stroke="#94a3b8"
            tick={{ fontSize: 12 }}
          />
          <YAxis stroke="#94a3b8" tick={{ fontSize: 12 }} allowDecimals={false} />
          <Tooltip
            contentStyle={{
              background: '#020617',
              border: '1px solid #334155',
              borderRadius: '0.75rem',
            }}
            labelFormatter={tooltipLabelFormatter}
          />
          <Legend />
          <Line type="monotone" dataKey="total" stroke="#38bdf8" strokeWidth={2} dot={false} />
          <Line type="monotone" dataKey="allowed" stroke="#22c55e" strokeWidth={2} dot={false} />
          <Line type="monotone" dataKey="blocked" stroke="#ef4444" strokeWidth={2} dot={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
