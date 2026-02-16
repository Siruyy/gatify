type SummaryCardProps = {
  title: string
  value: string
  subtitle?: string
}

export function SummaryCard({ title, value, subtitle }: SummaryCardProps) {
  return (
    <article className="rounded-2xl border border-slate-800 bg-slate-900 p-4 shadow-lg shadow-slate-950/30">
      <p className="text-xs uppercase tracking-wide text-slate-400">{title}</p>
      <p className="mt-2 text-3xl font-semibold text-white">{value}</p>
      {subtitle ? <p className="mt-1 text-sm text-slate-400">{subtitle}</p> : null}
    </article>
  )
}
