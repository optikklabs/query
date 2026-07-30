-- Versioned LLM prompts.
CREATE TABLE IF NOT EXISTS optikk.llm_prompts
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     tenant_id            BIGINT NOT NULL,
     name                 VARCHAR(200) NOT NULL,
     type                 ENUM('chat','text') NOT NULL DEFAULT 'chat',
     description          VARCHAR(1000) NULL,
     tags_json            JSON NOT NULL DEFAULT ('[]'),
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     updated_at           DATETIME NULL,
     created_by_user_id   BIGINT NULL,
     UNIQUE KEY uq_prompt_name (tenant_id, name),
     INDEX idx_prompt_tenant (tenant_id, updated_at)
  );

CREATE TABLE IF NOT EXISTS optikk.llm_prompt_versions
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     prompt_id            BIGINT NOT NULL,
     tenant_id            BIGINT NOT NULL,
     version              INT NOT NULL,
     template_json        JSON NOT NULL,
     variables_json       JSON NOT NULL DEFAULT ('[]'),
     notes                VARCHAR(1000) NULL,
     status               ENUM('draft','production','archived') NOT NULL DEFAULT 'draft',
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     created_by_user_id   BIGINT NULL,
     UNIQUE KEY uq_prompt_version (prompt_id, version),
     INDEX idx_pv_prompt (prompt_id, version),
     CONSTRAINT fk_pv_prompt FOREIGN KEY (prompt_id)
       REFERENCES optikk.llm_prompts (id) ON DELETE CASCADE
  );
