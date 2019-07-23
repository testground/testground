--
-- Migration 02: Initial indices (UP).
--

CREATE INDEX IF NOT EXISTS idx_test_runs_commit ON test_runs(commit_hash);

CREATE INDEX IF NOT EXISTS idx_metrics_name ON metrics(name);
