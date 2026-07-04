CREATE TABLE IF NOT EXISTS optikk.teams
  (
     id                BIGINT auto_increment PRIMARY KEY,
     org_name          VARCHAR(100) NOT NULL,
     name              VARCHAR(100) NOT NULL,
     slug              VARCHAR(50),
     description       VARCHAR(500),
     active            TINYINT(1) NOT NULL DEFAULT 1,
     color             VARCHAR(50),
     icon              VARCHAR(100),
     api_key           VARCHAR(80) NOT NULL UNIQUE,
     provisioning_status VARCHAR(20) NOT NULL DEFAULT 'pending',
     created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     INDEX idx_team_api_key (api_key),
     UNIQUE KEY uq_team_org_name (org_name, name)
  );

-- Idempotent upgrades for databases created before these columns changed.
ALTER TABLE optikk.teams MODIFY api_key VARCHAR(80) NOT NULL;
ALTER TABLE optikk.teams ADD COLUMN IF NOT EXISTS provisioning_status VARCHAR(20) NOT NULL DEFAULT 'pending';
