ALTER TABLE categories
  DROP CONSTRAINT IF EXISTS categories_type_check,
  ADD COLUMN IF NOT EXISTS color VARCHAR(7),
  ALTER COLUMN type TYPE TEXT;

UPDATE categories SET type = UPPER(type) WHERE type != UPPER(type);

ALTER TABLE categories
  ADD CONSTRAINT categories_type_check CHECK (type IN ('NEEDS', 'WANTS', 'SAVINGS'));

UPDATE transactions SET direction = 'INCOME' WHERE direction = 'in';
UPDATE transactions SET direction = 'EXPENSE' WHERE direction = 'out';
