-- This migration repairs schema drift inside the already-published Wiki
-- baseline. Rolling it back must not remove columns that migration 414 and the
-- application at version 454 already require.
SELECT 1;
