CREATE TABLE IF NOT EXISTS optikk.tenant
  (
     id                BIGINT auto_increment PRIMARY KEY,
     name              VARCHAR(100) NOT NULL,
     active            TINYINT(1) NOT NULL DEFAULT 1,
     api_key           VARCHAR(80) NOT NULL UNIQUE,
     created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     INDEX idx_tenant_api_key (api_key)
  );

-- Idempotent upgrades for databases created before these columns changed.
ALTER TABLE optikk.tenant MODIFY api_key VARCHAR(80) NOT NULL;

-- Tenant names are not globally unique: unrelated orgs may share a name.
-- Identity is (id, api_key). Drop the legacy unique constraint if present.
ALTER TABLE optikk.tenant DROP INDEX IF EXISTS uq_tenant_name;
