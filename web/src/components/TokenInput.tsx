import { useCallback, useEffect, useState } from 'react'
import { getRuntimeAdminToken, setRuntimeAdminToken } from '../lib/auth'

/**
 * TokenInput provides a UI for entering the admin API token.
 * The token is stored in-memory (via setRuntimeAdminToken) and never
 * persisted to localStorage/sessionStorage by default.
 */
export function TokenInput() {
  const [isOpen, setIsOpen] = useState(false)
  const [value, setValue] = useState('')
  const [saved, setSaved] = useState(false)

  const hasToken = !!getRuntimeAdminToken()

  useEffect(() => {
    if (isOpen) {
      setValue(getRuntimeAdminToken() ?? '')
      setSaved(false)
    }
  }, [isOpen])

  const handleSave = useCallback(() => {
    setRuntimeAdminToken(value)
    setSaved(true)
    setTimeout(() => {
      setIsOpen(false)
      // Force a re-render to update indicator dot
      window.dispatchEvent(new Event('gatify:token-changed'))
    }, 600)
  }, [value])

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        className="flex items-center gap-2 rounded-lg border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-slate-300 transition-colors hover:border-slate-600 hover:text-white"
        title={hasToken ? 'API token configured' : 'Set API token'}
      >
        <span
          className={`inline-block h-2 w-2 rounded-full ${hasToken ? 'bg-emerald-400' : 'bg-amber-400'}`}
        />
        Token
      </button>

      {isOpen && (
        <div className="absolute right-0 top-full z-50 mt-2 w-80 rounded-lg border border-slate-700 bg-slate-800 p-4 shadow-xl">
          <label className="mb-1 block text-xs font-medium text-slate-400" htmlFor="admin-token">
            Admin API Token
          </label>
          <input
            id="admin-token"
            type="password"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSave()}
            placeholder="Enter your admin token..."
            className="mb-3 w-full rounded-md border border-slate-600 bg-slate-900 px-3 py-2 text-sm text-white placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
          />
          <div className="flex items-center justify-between">
            <button
              type="button"
              onClick={() => setIsOpen(false)}
              className="text-xs text-slate-400 hover:text-white"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSave}
              className="rounded-md bg-cyan-600 px-3 py-1 text-xs font-medium text-white hover:bg-cyan-700"
            >
              {saved ? '✓ Saved' : 'Save'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
