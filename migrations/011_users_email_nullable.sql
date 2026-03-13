-- Make user email optional:
-- 1) convert empty strings to NULL
-- 2) allow NULL in users.email while preserving UNIQUE constraint behavior
UPDATE users
SET email = NULL
WHERE TRIM(COALESCE(email, '')) = '';

ALTER TABLE users
ALTER COLUMN email DROP NOT NULL;

