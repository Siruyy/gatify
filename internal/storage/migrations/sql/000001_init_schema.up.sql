CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS rules (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  pattern TEXT NOT NULL,
  methods TEXT[] NOT NULL,
  priority INTEGER NOT NULL DEFAULT 0,
  limit_value BIGINT NOT NULL CHECK (limit_value > 0),
  window_seconds INTEGER NOT NULL CHECK (window_seconds > 0),
  identify_by TEXT NOT NULL DEFAULT 'ip',
  header_name TEXT,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rate_limit_events (
  id BIGSERIAL NOT NULL,
  timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  client_id TEXT NOT NULL,
  method TEXT NOT NULL,
  path TEXT NOT NULL,
  allowed BOOLEAN NOT NULL,
  rule_id TEXT NOT NULL,
  limit_value BIGINT NOT NULL CHECK (limit_value > 0),
  remaining BIGINT NOT NULL,
  response_ms BIGINT NOT NULL CHECK (response_ms >= 0),
  PRIMARY KEY (timestamp, id)
);

SELECT create_hypertable(
  'rate_limit_events',
  'timestamp',
  if_not_exists => TRUE,
  migrate_data => TRUE
);

CREATE INDEX IF NOT EXISTS idx_rate_limit_events_timestamp_desc
  ON rate_limit_events (timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_rate_limit_events_rule_time
  ON rate_limit_events (rule_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_rate_limit_events_path_time
  ON rate_limit_events (path, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_rate_limit_events_allowed_time
  ON rate_limit_events (allowed, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_rate_limit_events_client_time
  ON rate_limit_events (client_id, timestamp DESC);

ALTER TABLE rate_limit_events SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'rule_id,method',
  timescaledb.compress_orderby = 'timestamp DESC'
);

SELECT add_compression_policy('rate_limit_events', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('rate_limit_events', INTERVAL '90 days', if_not_exists => TRUE);
