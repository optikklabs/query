CREATE TABLE IF NOT EXISTS optikk.device_codes
  (
     id             BIGINT auto_increment PRIMARY KEY,
     device_code    CHAR(64) NOT NULL,
     user_code      CHAR(9) NOT NULL,
     user_id        BIGINT NULL,
     approved_at    DATETIME NULL,
     consumed_at    DATETIME NULL,
     last_polled_at DATETIME NULL,
     expires_at     DATETIME NOT NULL,
     created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     UNIQUE KEY uq_device_code (device_code),
     UNIQUE KEY uq_user_code (user_code),
     INDEX idx_device_expires (expires_at)
  );
