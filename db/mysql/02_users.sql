CREATE TABLE IF NOT EXISTS optikk.users
  (
     id            BIGINT auto_increment PRIMARY KEY,
     email         VARCHAR(255) NOT NULL UNIQUE,
     password_hash VARCHAR(255),
     name          VARCHAR(100) NOT NULL,
     tenant_id     BIGINT NOT NULL,
     active        TINYINT(1) NOT NULL DEFAULT 1,
     is_admin      TINYINT(1) NOT NULL DEFAULT 0,
     created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     INDEX idx_user_email (email),
     INDEX idx_user_tenant (tenant_id)
  );

-- Idempotent upgrade for databases created before is_admin existed.
ALTER TABLE optikk.users ADD COLUMN IF NOT EXISTS is_admin TINYINT(1) NOT NULL DEFAULT 0;
