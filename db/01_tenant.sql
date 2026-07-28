CREATE TABLE IF NOT EXISTS optikk.tenant
  (
     id                BIGINT auto_increment PRIMARY KEY,
     name              VARCHAR(100) NOT NULL,
     active            TINYINT(1) NOT NULL DEFAULT 1,
                                                                         
                                                                         
                                                       
     api_key_hash      CHAR(64) NOT NULL UNIQUE,
     api_key_prefix    VARCHAR(16) NOT NULL DEFAULT '',
     account_status    VARCHAR(20) NOT NULL DEFAULT 'trialing',
     plan              VARCHAR(20) NOT NULL DEFAULT 'free',
     trial_ends_at     DATETIME NULL,
     created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     INDEX idx_tenant_trial (account_status, trial_ends_at)
  );
