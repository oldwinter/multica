DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'twin_profile'::regclass
          AND contype = 'p'
    ) THEN
        ALTER TABLE twin_profile
            ADD CONSTRAINT twin_profile_pkey PRIMARY KEY USING INDEX twin_profile_pkey;
    END IF;
END;
$$;
