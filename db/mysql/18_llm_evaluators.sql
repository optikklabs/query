CREATE TABLE IF NOT EXISTS optikk.llm_evaluators
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     tenant_id            BIGINT NOT NULL,
     name                 VARCHAR(200) NOT NULL,
     score_name           VARCHAR(200) NOT NULL,
     judge_model          VARCHAR(200) NULL,
     target               ENUM('traces','generations') NOT NULL DEFAULT 'generations',
     sampling_pct         INT NOT NULL DEFAULT 100,
     data_type            ENUM('numeric','boolean','categorical') NOT NULL DEFAULT 'numeric',
     categories_json      JSON NOT NULL DEFAULT ('[]'),
     prompt_template      TEXT NULL,
     enabled              TINYINT(1) NOT NULL DEFAULT 1,
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     updated_at           DATETIME NULL,
     created_by_user_id   BIGINT NULL,
     UNIQUE KEY uq_evaluator_name (tenant_id, name),
     INDEX idx_evaluator_tenant (tenant_id, enabled)
  );
