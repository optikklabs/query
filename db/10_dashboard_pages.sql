CREATE TABLE IF NOT EXISTS optikk.dashboard_pages
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     tenant_id              BIGINT NOT NULL,
     name                 VARCHAR(300) NOT NULL,
     description          VARCHAR(1000) NULL,
     icon                 VARCHAR(64) NOT NULL DEFAULT 'layout-grid',
     icon_color           VARCHAR(32) NOT NULL DEFAULT 'primary',
     tags_json            JSON NOT NULL DEFAULT ('[]'),
     is_favorite          TINYINT(1) NOT NULL DEFAULT 0,
     created_by_user_id   BIGINT NULL,
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     updated_at           DATETIME NULL,
     INDEX idx_dp_tenant (tenant_id)
  );
