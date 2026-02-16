/**
 * Escapes a string value for safe CSV output.
 * Prefixes formula-injection characters (=, +, -, @) with a single quote.
 * Wraps values containing commas, double-quotes, or newlines in double-quotes.
 */
export function escapeCsv(value: string): string {
  let sanitized = value

  if (/^[=+\-@]/.test(sanitized)) {
    sanitized = `'${sanitized}`
  }

  if (sanitized.includes(',') || sanitized.includes('"') || sanitized.includes('\n')) {
    return `"${sanitized.replaceAll('"', '""')}"`
  }

  return sanitized
}

/**
 * Validates a rate-limit rule form. Returns an error message string if
 * invalid, or null if the form is valid.
 */
export function validateRuleForm(form: {
  name: string
  pattern: string
  methods: string
  limit: string
  priority: string
  window_seconds: string
  identify_by: string
  header_name: string
}): string | null {
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

  const methods = form.methods
    .split(',')
    .map((m) => m.trim().toUpperCase())
    .filter(Boolean)
  if (methods.length === 0) {
    return 'At least one HTTP method is required.'
  }

  if (form.identify_by === 'header' && !form.header_name.trim()) {
    return 'Header name is required when identify by is set to header.'
  }

  return null
}
