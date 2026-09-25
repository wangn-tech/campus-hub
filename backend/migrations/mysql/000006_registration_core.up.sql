CREATE TABLE IF NOT EXISTS registration_status_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    registration_id BIGINT UNSIGNED NOT NULL,
    from_status TINYINT NOT NULL,
    to_status TINYINT NOT NULL,
    operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    operator_type TINYINT NOT NULL DEFAULT 1,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    KEY idx_registration_created (registration_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
