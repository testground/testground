--
-- Migration 02: Initial indices (DOWN).
--

DROP INDEX IF EXISTS idx_test_runs_commit;

DROP INDEX IF EXISTS idx_metrics_name;