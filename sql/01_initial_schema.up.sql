--
-- Migration 01: Initial schema setup (UP).
--

-- This table contains test plans, which in turn contain test cases.
CREATE TABLE IF NOT EXISTS test_plans
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    desc       TEXT    NULL,

    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,

    -- runner specifies the runner strategy for this test plan (e.g. canary, private_network, etc.)
    runner     INTEGER NOT NULL,

    UNIQUE (name)
);

-- This table contains test cases, each of which is associated with a test plan.
CREATE TABLE IF NOT EXISTS test_cases
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    desc         TEXT    NULL,
    test_plan_id INTEGER NOT NULL,

    created_at   DATETIME,
    updated_at   DATETIME,
    deleted_at   DATETIME,

    UNIQUE (id, test_plan_id),
    FOREIGN KEY (test_plan_id) REFERENCES test_plans (id)
);

-- This table tracks runs of test plans.
CREATE TABLE IF NOT EXISTS test_runs
(
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    test_plan_id     INTEGER  NOT NULL,
    commit_hash      TEXT     NOT NULL,

    -- Why was this run launched? GitHub commit, GitHub comment, manually via the API?
    reason           TEXT     NOT NULL,

    -- How many iterations we've scheduled.
    total_iterations INTEGER  NOT NULL,
    started_at       DATETIME NOT NULL,
    ended_at         DATETIME NULL,

    created_at       DATETIME,
    updated_at       DATETIME,
    deleted_at       DATETIME,

    FOREIGN KEY (test_plan_id) REFERENCES test_plans (id)
);


-- This table tracks test run iterations.
CREATE TABLE IF NOT EXISTS test_iterations
(
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    test_run_id INTEGER  NOT NULL,
    number      INTEGER  NOT NULL,
    started_at  DATETIME NOT NULL,
    ended_at    DATETIME NULL,

    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME,

    FOREIGN KEY (test_run_id) REFERENCES test_runs (id),
    UNIQUE (test_run_id, number)
);

-- This table is a catalogue of all metrics we're tracking.
CREATE TABLE IF NOT EXISTS metrics
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    desc         TEXT    NULL,
    unit         TEXT    NULL,
    improve_dir  INTEGER NULL,
    ui_order     INTEGER NOT NULL,

    -- If NULL, this is a cross-cutting metric.
    test_case_id INTEGER NULL,

    created_at   DATETIME,
    updated_at   DATETIME,
    deleted_at   DATETIME,

    UNIQUE (name, test_case_id),
    FOREIGN KEY (test_case_id) REFERENCES test_cases (id)
);

-- This table stores observations (data points) on defined metrics.
CREATE TABLE IF NOT EXISTS results
(
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_id         INTEGER NOT NULL,
    test_iteration_id INTEGER NOT NULL,
    value             REAL    NULL,
    recorded_at       INTEGER NOT NULL,

    created_at        DATETIME,
    updated_at        DATETIME,
    deleted_at        DATETIME,

    FOREIGN KEY (metric_id) REFERENCES metrics (id),
    FOREIGN KEY (test_iteration_id) REFERENCES test_iterations (id),
    UNIQUE (metric_id, test_iteration_id)
);

-- This table caches repository commits.
CREATE TABLE IF NOT EXISTS commits
(
    sha         TEXT PRIMARY KEY,
    repo_url    TEXT      NOT NULL,
    author      TEXT      NOT NULL,
    branch      TEXT      NOT NULL,
    message     TEXT      NOT NULL,
    commited_at TIMESTAMP NOT NULL,

    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME
);

