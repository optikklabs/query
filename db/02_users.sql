CREATE TABLE IF NOT EXISTS optikk.users
  (
     id                BIGINT AUTO_INCREMENT PRIMARY KEY,
     email             VARCHAR(255) NOT NULL UNIQUE,
     password_hash     VARCHAR(255),
     name              VARCHAR(100) NOT NULL,
     tenant_id         BIGINT NOT NULL,
     active            TINYINT(1) NOT NULL DEFAULT 1,
     role              VARCHAR(20) NOT NULL DEFAULT 'member',
     terms_accepted_at DATETIME,
     terms_version     VARCHAR(20) NOT NULL DEFAULT '',
     created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     INDEX idx_user_email (email),
     INDEX idx_user_tenant (tenant_id)
  );
