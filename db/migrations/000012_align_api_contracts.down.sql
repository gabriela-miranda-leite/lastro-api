UPDATE transactions SET direction = 'in' WHERE direction = 'INCOME';
UPDATE transactions SET direction = 'out' WHERE direction = 'EXPENSE';

ALTER TABLE categories
  DROP CONSTRAINT IF EXISTS categories_type_check;

UPDATE categories SET type = LOWER(type) WHERE type != LOWER(type);

ALTER TABLE categories
  ADD CONSTRAINT categories_type_check CHECK (type IN ('necessity', 'leisure', 'investment', 'saving')),
  DROP COLUMN IF EXISTS color;
