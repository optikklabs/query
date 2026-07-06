-- Trial/billing lifecycle for tenants. account_status is the source of truth;
-- `active` (01_tenant.sql) stays the ingest-facing projection of it.
ALTER TABLE optikk.tenant
  ADD COLUMN IF NOT EXISTS account_status VARCHAR(20) NOT NULL DEFAULT 'trialing';
ALTER TABLE optikk.tenant
  ADD COLUMN IF NOT EXISTS plan           VARCHAR(20) NOT NULL DEFAULT 'free';
ALTER TABLE optikk.tenant
  ADD COLUMN IF NOT EXISTS trial_ends_at  DATETIME NULL;

-- Supports the trial sweeper's (status, expiry) range scan.
CREATE INDEX IF NOT EXISTS idx_tenant_trial
  ON optikk.tenant (account_status, trial_ends_at);
