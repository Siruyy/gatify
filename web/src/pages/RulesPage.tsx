import { useRules } from '../hooks/useDashboardData'

export function RulesPage() {
  const rulesQuery = useRules()

  if (rulesQuery.isLoading) {
    return <p className="text-slate-300">Loading rules...</p>
  }

  if (rulesQuery.isError || !rulesQuery.data) {
    return (
      <div className="rounded-xl border border-red-500/30 bg-red-950/30 p-4 text-red-200">
        Failed to load rules. Ensure `VITE_API_BASE_URL` and runtime admin auth are configured.
      </div>
    )
  }

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-2xl font-semibold text-white">Rules Management</h2>
        <p className="mt-1 text-sm text-slate-400">Starter table scaffold for create/edit/toggle flows.</p>
      </div>

      <div className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-900 shadow-lg shadow-slate-950/30">
        <table className="min-w-full divide-y divide-slate-800 text-sm">
          <thead className="bg-slate-900/80 text-left text-slate-300">
            <tr>
              <th scope="col" className="px-4 py-3 font-medium">Name</th>
              <th scope="col" className="px-4 py-3 font-medium">Pattern</th>
              <th scope="col" className="px-4 py-3 font-medium">Methods</th>
              <th scope="col" className="px-4 py-3 font-medium">Limit</th>
              <th scope="col" className="px-4 py-3 font-medium">Window</th>
              <th scope="col" className="px-4 py-3 font-medium">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800 text-slate-100">
            {rulesQuery.data.map((rule) => (
              <tr key={rule.id}>
                <td className="px-4 py-3">{rule.name}</td>
                <td className="px-4 py-3 text-slate-300">{rule.pattern}</td>
                <td className="px-4 py-3">{(rule.methods ?? ['*']).join(', ')}</td>
                <td className="px-4 py-3">{rule.limit}</td>
                <td className="px-4 py-3">{rule.window_seconds}s</td>
                <td className="px-4 py-3">
                  <span
                    className={[
                      'inline-flex rounded-full px-2.5 py-1 text-xs font-medium',
                      rule.enabled ? 'bg-emerald-500/20 text-emerald-300' : 'bg-slate-700 text-slate-300',
                    ].join(' ')}
                  >
                    {rule.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
