CREATE TABLE IF NOT EXISTS optikk.dashboards
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     page_id              BIGINT NOT NULL,
     tenant_id              BIGINT NOT NULL,
     title                VARCHAR(300) NULL,
     panel_type           VARCHAR(64) NOT NULL,
     layout_variant       VARCHAR(64) NULL,
     spec_json            JSON NOT NULL,
     layout_json          JSON NOT NULL,
     position             INT NOT NULL DEFAULT 0,
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     updated_at           DATETIME NULL,
     INDEX idx_d_page (page_id),
     CONSTRAINT fk_d_page FOREIGN KEY (page_id)
       REFERENCES optikk.dashboard_pages (id) ON DELETE CASCADE
  );
