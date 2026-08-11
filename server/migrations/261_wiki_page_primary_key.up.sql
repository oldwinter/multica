DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'wiki_page'::regclass
          AND contype = 'p'
    ) THEN
        ALTER TABLE wiki_page
            ADD CONSTRAINT wiki_page_pkey PRIMARY KEY USING INDEX wiki_page_pkey;
    END IF;
END $$;
