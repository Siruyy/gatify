import { useMemo, useState } from 'react'
import {
  useCreateRule,
  useDeleteRule,
  useRules,
  useUpdateRule,
  type IdentifyBy,
  type Rule,
  type RulePayload,
} from '../hooks/useDashboardData'

type FormState = {
  name: string
  pattern: string
  methods: string
  priority: string
  limit: string
  window_seconds: string
  identify_by: IdentifyBy
  header_name: string
  enabled: boolean
}

const defaultFormState: FormState = {
  name: '',
  pattern: '/api/*',
  methods: 'GET',
  priority: '1',
  limit: '100',
  window_seconds: '60',
  identify_by: 'ip',
  header_name: '',
  enabled: true,
}

function parseMethods(value: string): string[] {
  return value
    .split(',')
    .map((m) => m.trim().toUpperCase())
    .filter(Boolean)
}

function toFormState(rule: Rule): FormState {
  return {
    name: rule.name,
    pattern: rule.pattern,
    methods: rule.methods?.length ? rule.methods.join(', ') : 'GET',
    priority: String(rule.priority),
    limit: String(rule.limit),
    window_seconds: String(rule.window_seconds),
    identify_by: rule.identify_by === 'header' ? 'header' : 'ip',
    header_name: rule.header_name ?? '',
    enabled: rule.enabled,
  }
}

function ruleToPayload(rule: Rule): RulePayload {
  return {
    name: rule.name,
    pattern: rule.pattern,
    methods: rule.methods ?? [],
    priority: rule.priority,
    limit: rule.limit,
    window_seconds: rule.window_seconds,
    identify_by: rule.identify_by === 'header' ? 'header' : 'ip',
    header_name: rule.header_name ?? '',
    enabled: rule.enabled,
  }
}

function validateForm(form: FormState): string | null {
  if (!form.name.trim()) {
    return 'Name is required.'
  }
  if (!form.pattern.trim()) {
    return 'Pattern is required.'
  }

  const limit = Number(form.limit)
  if (!Number.isInteger(limit) || limit <= 0) {
    return 'Limit must be a positive integer.'
  }

  const priority = Number(form.priority)
  if (!Number.isInteger(priority) || priority < 0) {
    return 'Priority must be a non-negative integer.'
  }

  const windowSeconds = Number(form.window_seconds)
  if (!Number.isInteger(windowSeconds) || windowSeconds <= 0) {
    return 'Window must be a positive integer (seconds).'
  }

  const methods = parseMethods(form.methods)
  if (methods.length === 0) {
    return 'At least one HTTP method is required.'
  }

  if (form.identify_by === 'header' && !form.header_name.trim()) {
    return 'Header name is required when identify by is set to header.'
  }

  return null
}

function buildPayload(form: FormState): RulePayload {
  return {
    name: form.name.trim(),
    pattern: form.pattern.trim(),
    methods: parseMethods(form.methods),
    priority: Number(form.priority),
    limit: Number(form.limit),
    window_seconds: Number(form.window_seconds),
    identify_by: form.identify_by,
    header_name: form.identify_by === 'header' ? form.header_name.trim() : undefined,
    enabled: form.enabled,
  }
}

type ConfirmModalProps = {
  open: boolean
  title: string
  description: string
  confirmLabel?: string
  isLoading?: boolean
  onCancel: () => void
  onConfirm: () => void
}

function ConfirmModal({
  open,
  title,
  description,
  confirmLabel = 'Confirm',
  isLoading = false,
  onCancel,
  onConfirm,
}: ConfirmModalProps) {
  if (!open) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 px-4">
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-5 shadow-xl shadow-slate-950/60">
        <h3 className="text-lg font-semibold text-white">{title}</h3>
        <p className="mt-2 text-sm text-slate-300">{description}</p>

        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            disabled={isLoading}
            className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-200 transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={isLoading}
            className="rounded-lg bg-red-500 px-4 py-2 text-sm font-medium text-white transition hover:bg-red-400 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

export function RulesPage() {
  const rulesQuery = useRules()
  const createRule = useCreateRule()
  const updateRule = useUpdateRule()
  const deleteRule = useDeleteRule()

  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'enabled' | 'disabled'>('all')
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<Rule | null>(null)
  const [form, setForm] = useState<FormState>(defaultFormState)
  const [formError, setFormError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Rule | null>(null)

  const isMutating = createRule.isPending || updateRule.isPending || deleteRule.isPending

  const filteredRules = useMemo(() => {
    const rules = rulesQuery.data ?? []
    const needle = search.trim().toLowerCase()

    return rules.filter((rule) => {
      const matchesSearch =
        needle.length === 0 ||
        rule.name.toLowerCase().includes(needle) ||
        rule.pattern.toLowerCase().includes(needle) ||
        (rule.methods ?? []).join(',').toLowerCase().includes(needle)

      const matchesStatus =
        statusFilter === 'all' ||
        (statusFilter === 'enabled' && rule.enabled) ||
        (statusFilter === 'disabled' && !rule.enabled)

      return matchesSearch && matchesStatus
    })
  }, [rulesQuery.data, search, statusFilter])

  const openCreateForm = () => {
    setEditingRule(null)
    setForm(defaultFormState)
    setFormError(null)
    setActionError(null)
    setIsFormOpen(true)
  }

  const openEditForm = (rule: Rule) => {
    setEditingRule(rule)
    setForm(toFormState(rule))
    setFormError(null)
    setActionError(null)
    setIsFormOpen(true)
  }

  const closeForm = () => {
    setIsFormOpen(false)
    setEditingRule(null)
    setFormError(null)
  }

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setFormError(null)
    setActionError(null)

    const validationError = validateForm(form)
    if (validationError) {
      setFormError(validationError)
      return
    }

    const payload = buildPayload(form)

    try {
      if (editingRule) {
        await updateRule.mutateAsync({ id: editingRule.id, payload })
      } else {
        await createRule.mutateAsync(payload)
      }
      closeForm()
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to save rule.'
      setActionError(message)
    }
  }

  const handleToggle = async (rule: Rule) => {
    setActionError(null)
    try {
      await updateRule.mutateAsync({
        id: rule.id,
        payload: {
          ...ruleToPayload(rule),
          enabled: !rule.enabled,
        },
      })
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to update rule status.'
      setActionError(message)
    }
  }

  const handleDelete = (rule: Rule) => {
    setActionError(null)
    setDeleteTarget(rule)
  }

  const confirmDelete = async () => {
    if (!deleteTarget) {
      return
    }

    try {
      await deleteRule.mutateAsync(deleteTarget.id)
      setDeleteTarget(null)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to delete rule.'
      setActionError(message)
    }
  }

  if (rulesQuery.isLoading) {
    return <p className="text-slate-300">Loading rules...</p>
  }

  if (rulesQuery.isError) {
    return (
      <div className="rounded-xl border border-red-500/30 bg-red-950/30 p-4 text-red-200">
        Failed to load rules. Ensure `VITE_API_BASE_URL` and runtime admin auth are configured.
      </div>
    )
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-white">Rules Management</h2>
          <p className="mt-1 text-sm text-slate-400">Create, edit, enable/disable, and remove rate-limit rules.</p>
        </div>
        <button
          type="button"
          onClick={openCreateForm}
          className="rounded-lg bg-cyan-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-cyan-400"
        >
          New Rule
        </button>
      </div>

      <div className="grid gap-3 rounded-xl border border-slate-800 bg-slate-900/60 p-4 sm:grid-cols-[minmax(0,1fr)_180px]">
        <input
          type="text"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search by name, pattern, or methods"
          className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 placeholder:text-slate-500 focus:ring"
        />
        <select
          value={statusFilter}
          onChange={(event) => setStatusFilter(event.target.value as 'all' | 'enabled' | 'disabled')}
          className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
        >
          <option value="all">All statuses</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
        </select>
      </div>

      {actionError ? (
        <div className="rounded-xl border border-red-500/30 bg-red-950/30 p-4 text-red-200">{actionError}</div>
      ) : null}

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
              <th scope="col" className="px-4 py-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800 text-slate-100">
            {filteredRules.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-slate-400">
                  No rules found. Adjust filters or create a new rule.
                </td>
              </tr>
            ) : (
              filteredRules.map((rule) => (
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
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => openEditForm(rule)}
                        disabled={isMutating}
                        className="rounded-md border border-slate-700 px-2.5 py-1 text-xs text-slate-200 transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        onClick={() => handleToggle(rule)}
                        disabled={isMutating}
                        className="rounded-md border border-slate-700 px-2.5 py-1 text-xs text-slate-200 transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {rule.enabled ? 'Disable' : 'Enable'}
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDelete(rule)}
                        disabled={isMutating}
                        className="rounded-md border border-red-500/40 px-2.5 py-1 text-xs text-red-200 transition hover:bg-red-500/10 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {isFormOpen ? (
        <div className="rounded-2xl border border-slate-800 bg-slate-900 p-5 shadow-lg shadow-slate-950/30">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-lg font-semibold text-white">{editingRule ? 'Edit Rule' : 'Create Rule'}</h3>
            <button
              type="button"
              onClick={closeForm}
              className="rounded-md border border-slate-700 px-2.5 py-1 text-xs text-slate-200 transition hover:bg-slate-800"
            >
              Cancel
            </button>
          </div>

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="space-y-1 text-sm text-slate-200">
                <span>Name</span>
                <input
                  type="text"
                  value={form.name}
                  onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                />
              </label>

              <label className="space-y-1 text-sm text-slate-200">
                <span>Path Pattern</span>
                <input
                  type="text"
                  value={form.pattern}
                  onChange={(event) => setForm((prev) => ({ ...prev, pattern: event.target.value }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                />
              </label>

              <label className="space-y-1 text-sm text-slate-200">
                <span>Methods (comma-separated)</span>
                <input
                  type="text"
                  value={form.methods}
                  onChange={(event) => setForm((prev) => ({ ...prev, methods: event.target.value }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                />
              </label>

              <label className="space-y-1 text-sm text-slate-200">
                <span>Priority</span>
                <input
                  type="number"
                  value={form.priority}
                  onChange={(event) => setForm((prev) => ({ ...prev, priority: event.target.value }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                />
              </label>

              <label className="space-y-1 text-sm text-slate-200">
                <span>Limit</span>
                <input
                  type="number"
                  min={1}
                  value={form.limit}
                  onChange={(event) => setForm((prev) => ({ ...prev, limit: event.target.value }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                />
              </label>

              <label className="space-y-1 text-sm text-slate-200">
                <span>Window (seconds)</span>
                <input
                  type="number"
                  min={1}
                  value={form.window_seconds}
                  onChange={(event) => setForm((prev) => ({ ...prev, window_seconds: event.target.value }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                />
              </label>

              <label className="space-y-1 text-sm text-slate-200">
                <span>Identify By</span>
                <select
                  value={form.identify_by}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      identify_by: event.target.value as IdentifyBy,
                    }))
                  }
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                >
                  <option value="ip">IP Address</option>
                  <option value="header">Header</option>
                </select>
              </label>

              <label className="flex items-end gap-2 pb-2 text-sm text-slate-200">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(event) => setForm((prev) => ({ ...prev, enabled: event.target.checked }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="h-4 w-4 rounded border-slate-600 bg-slate-950"
                />
                Enabled
              </label>
            </div>

            {form.identify_by === 'header' ? (
              <label className="space-y-1 text-sm text-slate-200">
                <span>Header Name</span>
                <input
                  type="text"
                  value={form.header_name}
                  onChange={(event) => setForm((prev) => ({ ...prev, header_name: event.target.value }))}
                  aria-invalid={Boolean(formError)}
                  aria-describedby={formError ? 'rules-form-error' : undefined}
                  className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none ring-cyan-500/40 focus:ring"
                />
              </label>
            ) : null}

            {formError ? (
              <p id="rules-form-error" className="text-sm text-red-300">
                {formError}
              </p>
            ) : null}
            {actionError ? <p className="text-sm text-red-300">{actionError}</p> : null}

            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={closeForm}
                className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-200 transition hover:bg-slate-800"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isMutating}
                className="rounded-lg bg-cyan-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {editingRule ? 'Save Changes' : 'Create Rule'}
              </button>
            </div>
          </form>
        </div>
      ) : null}

      <ConfirmModal
        open={Boolean(deleteTarget)}
        title="Delete rule"
        description={
          deleteTarget
            ? `Delete rule "${deleteTarget.name}"? This action cannot be undone.`
            : 'Delete this rule? This action cannot be undone.'
        }
        confirmLabel="Delete"
        isLoading={deleteRule.isPending}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={confirmDelete}
      />
    </section>
  )
}
