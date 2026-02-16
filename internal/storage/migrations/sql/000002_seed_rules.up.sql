INSERT INTO rules (
  id,
  name,
  path_pattern,
  http_method,
  limit_value,
  window_seconds,
  enabled,
  description
)
VALUES (
  'default-global',
  'Default global rule',
  '*',
  '*',
  100,
  60,
  TRUE,
  'Seed fallback rule for local testing'
)
ON CONFLICT (id) DO NOTHING;
