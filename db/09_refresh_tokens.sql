CREATE TABLE IF NOT EXISTS optikk.refresh_tokens
  (
     id          BIGINT auto_increment PRIMARY KEY,
     user_id     BIGINT NOT NULL,
     family_id   CHAR(36) NOT NULL,
     token_hash  CHAR(64) NOT NULL,
     expires_at  DATETIME NOT NULL,
     revoked_at  DATETIME NULL,
     created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     UNIQUE KEY uq_refresh_token_hash (token_hash),
     INDEX idx_refresh_user (user_id),
     INDEX idx_refresh_family (family_id),
     INDEX idx_refresh_expires (expires_at)
  );
