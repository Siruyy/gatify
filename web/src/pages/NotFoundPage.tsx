import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-slate-950 px-4 text-slate-100">
      <p className="text-sm uppercase tracking-wide text-slate-400">404</p>
      <h1 className="text-3xl font-bold">Page not found</h1>
      <p className="text-slate-400">The route you requested does not exist in the dashboard scaffold.</p>
      <Link
        to="/dashboard"
        className="rounded-lg bg-cyan-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-cyan-400"
      >
        Go to dashboard
      </Link>
    </div>
  )
}
