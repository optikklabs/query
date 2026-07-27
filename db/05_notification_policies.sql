CREATE TABLE IF NOT EXISTS optikk.notification_policies
  (
     id            BIGINT AUTO_INCREMENT PRIMARY KEY,
     tenant_id       BIGINT NOT NULL,
     name          VARCHAR(200) NOT NULL,
     match_dsl     VARCHAR(500) NOT NULL,
     actions_json  JSON NOT NULL,
     hits_30d      INT NOT NULL DEFAULT 0,
     last_used_at  DATETIME NULL,
     enabled       TINYINT(1) NOT NULL DEFAULT 1,
     position      INT NOT NULL DEFAULT 0,
     created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     updated_at    DATETIME NULL,
     INDEX idx_np_tenant_position (tenant_id, position)
  );
