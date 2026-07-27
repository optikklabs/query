CREATE TABLE IF NOT EXISTS optikk.monitor_state
  (
     monitor_id           BIGINT PRIMARY KEY,
     status               ENUM('alert','warn','ok','no_data') NOT NULL DEFAULT 'no_data',
     current_value        DOUBLE NULL,
     last_evaluated_at    DATETIME NULL,
     next_evaluation_at   DATETIME NOT NULL,
     triggered_at         DATETIME NULL,
     last_notified_at     DATETIME NULL,
     evaluation_count     BIGINT NOT NULL DEFAULT 0,
     acked_by_user_id     BIGINT NULL,
     acked_at             DATETIME NULL,
     -- Work-claiming lease: an evaluator replica stamps claimed_by/until
     -- before evaluating so replicas never evaluate the same monitor twice.
     claimed_by           CHAR(36) NULL,
     claimed_until        DATETIME NULL,
     INDEX idx_ms_due (next_evaluation_at, claimed_until),
     INDEX idx_ms_claim (claimed_by)
  );
