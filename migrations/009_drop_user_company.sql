-- Remove deprecated optional profile field: company.

ALTER TABLE users
  DROP COLUMN IF EXISTS company;
