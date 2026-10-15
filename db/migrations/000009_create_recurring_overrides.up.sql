CREATE TABLE recurring_overrides (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recurring_id    UUID NOT NULL REFERENCES recurring_expenses(id) ON DELETE CASCADE,
  year            INT NOT NULL,
  month           INT NOT NULL CHECK (month BETWEEN 1 AND 12),
  amount_override NUMERIC(12,2),
  active          BOOLEAN NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (recurring_id, year, month)
);
