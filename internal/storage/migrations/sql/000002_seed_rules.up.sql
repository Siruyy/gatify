INSERT INTO rules (
  id,
  name,
  pattern,
  methods,
  priority,
  limit_value,
  window_seconds,
  identify_by,
  enabled,
  description
)
VALUES (
  'default-global',
  'Default global rule',
  '/*',
  ARRAY['*'],
  0,
  100,
  60,
  'ip',
  TRUE,
  'Seed fallback rule for local testing'
)
ON CONFLICT (id) DO NOTHING;
