SELECT remove_retention_policy('rate_limit_events', if_exists => TRUE);
SELECT remove_compression_policy('rate_limit_events', if_exists => TRUE);

DROP TABLE IF EXISTS rate_limit_events;
DROP TABLE IF EXISTS rules;
