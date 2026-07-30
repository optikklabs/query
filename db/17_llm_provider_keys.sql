-- Encrypted LLM provider credentials.
CREATE TABLE IF NOT EXISTS optikk.llm_provider_keys
  (
     id                   BIGINT AUTO_INCREMENT PRIMARY KEY,
     tenant_id            BIGINT NOT NULL,
     provider             ENUM('openai','anthropic','mistral') NOT NULL,
     label                VARCHAR(200) NOT NULL,
     key_ciphertext       VARBINARY(1024) NOT NULL,
     nonce                VARBINARY(64) NOT NULL,
     key_last4            VARCHAR(8) NOT NULL,
     created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     created_by_user_id   BIGINT NULL,
     UNIQUE KEY uq_provider_key (tenant_id, provider, label),
     INDEX idx_pk_tenant (tenant_id, provider)
  );
