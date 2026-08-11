-- Single statement: CREATE INDEX CONCURRENTLY cannot run inside a transaction.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS twin_profile_pkey
    ON twin_profile (id);
