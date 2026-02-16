import { describe, expect, it } from 'vitest'
import { escapeCsv, validateRuleForm } from './utils'

describe('escapeCsv', () => {
  it('returns plain values unmodified', () => {
    expect(escapeCsv('hello')).toBe('hello')
  })

  it('prefixes formula-injection characters with a single quote', () => {
    expect(escapeCsv('=SUM(A1)')).toBe("'=SUM(A1)")
    expect(escapeCsv('+cmd')).toBe("'+cmd")
    expect(escapeCsv('-danger')).toBe("'-danger")
    expect(escapeCsv('@import')).toBe("'@import")
  })

  it('wraps values containing commas in double quotes', () => {
    expect(escapeCsv('foo,bar')).toBe('"foo,bar"')
  })

  it('wraps values containing double quotes and escapes inner quotes', () => {
    expect(escapeCsv('say "hello"')).toBe('"say ""hello"""')
  })

  it('wraps values containing newlines in double quotes', () => {
    expect(escapeCsv('line1\nline2')).toBe('"line1\nline2"')
  })

  it('handles combined special chars', () => {
    // Formula char + comma → quoted with prefix
    expect(escapeCsv('=a,b')).toBe("\"'=a,b\"")
  })
})

describe('validateRuleForm', () => {
  const validForm = {
    name: 'test-rule',
    pattern: '/api/*',
    methods: 'GET, POST',
    limit: '100',
    priority: '1',
    window_seconds: '60',
    identify_by: 'ip',
    header_name: '',
  }

  it('returns null for a valid form', () => {
    expect(validateRuleForm(validForm)).toBeNull()
  })

  it('requires name', () => {
    expect(validateRuleForm({ ...validForm, name: '' })).toBe('Name is required.')
    expect(validateRuleForm({ ...validForm, name: '   ' })).toBe('Name is required.')
  })

  it('requires pattern', () => {
    expect(validateRuleForm({ ...validForm, pattern: '' })).toBe('Pattern is required.')
  })

  it('requires a positive integer limit', () => {
    expect(validateRuleForm({ ...validForm, limit: '0' })).toBe('Limit must be a positive integer.')
    expect(validateRuleForm({ ...validForm, limit: '-5' })).toBe('Limit must be a positive integer.')
    expect(validateRuleForm({ ...validForm, limit: 'abc' })).toBe('Limit must be a positive integer.')
  })

  it('requires a non-negative integer priority', () => {
    expect(validateRuleForm({ ...validForm, priority: '-1' })).toBe(
      'Priority must be a non-negative integer.',
    )
  })

  it('allows zero priority', () => {
    expect(validateRuleForm({ ...validForm, priority: '0' })).toBeNull()
  })

  it('requires a positive integer window', () => {
    expect(validateRuleForm({ ...validForm, window_seconds: '0' })).toBe(
      'Window must be a positive integer (seconds).',
    )
  })

  it('requires at least one HTTP method', () => {
    expect(validateRuleForm({ ...validForm, methods: '' })).toBe(
      'At least one HTTP method is required.',
    )
  })

  it('requires header_name when identify_by is header', () => {
    expect(
      validateRuleForm({ ...validForm, identify_by: 'header', header_name: '' }),
    ).toBe('Header name is required when identify by is set to header.')
  })

  it('passes when header_name is provided for header identify_by', () => {
    expect(
      validateRuleForm({ ...validForm, identify_by: 'header', header_name: 'X-Api-Key' }),
    ).toBeNull()
  })
})
