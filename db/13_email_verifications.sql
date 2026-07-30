-- Email verification tokens.
CREATE TABLE IF NOT EXISTS optikk.email_verifications (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  token_hash CHAR(64) NOT NULL UNIQUE,
  expires_at DATETIME NOT NULL,
  consumed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_email_verification_user (user_id),
  INDEX idx_email_verification_expiry (expires_at)
);
