-- Replace the platform-wide is_admin flag with a tenant-scoped role.
-- Idempotent: the migrator re-runs every file on each boot.

ALTER TABLE optikk.users
  ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'member';

-- Backfill role from is_admin only while that column still exists, so the
-- statement stays valid on later boots after is_admin has been dropped.
SET @backfill := IF(
  EXISTS(SELECT 1 FROM information_schema.columns
         WHERE table_schema = 'optikk' AND table_name = 'users'
           AND column_name = 'is_admin'),
  'UPDATE optikk.users SET role = ''admin'' WHERE is_admin = 1',
  'SELECT 1');
PREPARE backfill_stmt FROM @backfill;
EXECUTE backfill_stmt;
DEALLOCATE PREPARE backfill_stmt;

ALTER TABLE optikk.users DROP COLUMN IF EXISTS is_admin;
