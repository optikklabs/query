-- LLM evaluation datasets and runs.
CREATE TABLE IF NOT EXISTS optikk.llm_datasets
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     tenant_id            BIGINT NOT NULL,
     name                 VARCHAR(200) NOT NULL,
     description          VARCHAR(1000) NULL,
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     updated_at           DATETIME NULL,
     created_by_user_id   BIGINT NULL,
     UNIQUE KEY uq_dataset_name (tenant_id, name),
     INDEX idx_dataset_tenant (tenant_id, updated_at)
  );

CREATE TABLE IF NOT EXISTS optikk.llm_dataset_items
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     dataset_id           BIGINT NOT NULL,
     tenant_id            BIGINT NOT NULL,
     input_json           JSON NOT NULL,
     expected_output_json JSON NULL,
     metadata_json        JSON NOT NULL DEFAULT ('{}'),
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     INDEX idx_di_dataset (dataset_id),
     CONSTRAINT fk_di_dataset FOREIGN KEY (dataset_id)
       REFERENCES optikk.llm_datasets (id) ON DELETE CASCADE
  );

CREATE TABLE IF NOT EXISTS optikk.llm_experiment_runs
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     dataset_id           BIGINT NOT NULL,
     tenant_id            BIGINT NOT NULL,
     name                 VARCHAR(200) NOT NULL,
     provider             VARCHAR(64) NOT NULL,
     model                VARCHAR(200) NOT NULL,
     prompt_version_id    BIGINT NULL,
     params_json          JSON NOT NULL DEFAULT ('{}'),
     status               ENUM('running','completed','failed') NOT NULL DEFAULT 'running',
     item_count           INT NOT NULL DEFAULT 0,
     avg_scores_json      JSON NOT NULL DEFAULT ('{}'),
     total_cost_usd       DOUBLE NOT NULL DEFAULT 0,
     avg_latency_ms       DOUBLE NOT NULL DEFAULT 0,
     error                VARCHAR(1000) NULL,
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     completed_at         DATETIME NULL,
     INDEX idx_run_dataset (dataset_id, created_at),
     CONSTRAINT fk_run_dataset FOREIGN KEY (dataset_id)
       REFERENCES optikk.llm_datasets (id) ON DELETE CASCADE
  );

CREATE TABLE IF NOT EXISTS optikk.llm_experiment_run_items
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     run_id               BIGINT NOT NULL,
     tenant_id            BIGINT NOT NULL,
     dataset_item_id      BIGINT NOT NULL,
     output_json          JSON NULL,
     latency_ms           INT NOT NULL DEFAULT 0,
     cost_usd             DOUBLE NOT NULL DEFAULT 0,
     scores_json          JSON NOT NULL DEFAULT ('{}'),
     error                VARCHAR(1000) NULL,
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     INDEX idx_ri_run (run_id),
     CONSTRAINT fk_ri_run FOREIGN KEY (run_id)
       REFERENCES optikk.llm_experiment_runs (id) ON DELETE CASCADE
  );
