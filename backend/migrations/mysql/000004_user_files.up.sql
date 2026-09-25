ALTER TABLE users ADD COLUMN avatar_file_id BIGINT UNSIGNED NULL AFTER nickname, ADD KEY idx_users_avatar_file (avatar_file_id);
ALTER TABLE student_verifications
    ADD COLUMN ocr_platform VARCHAR(20) NOT NULL DEFAULT '' AFTER back_file_id,
    ADD COLUMN ocr_confidence DECIMAL(5,2) NULL AFTER ocr_platform,
    ADD COLUMN ocr_raw_json JSON NULL AFTER ocr_confidence,
    ADD COLUMN ocr_completed_at DATETIME(3) NULL AFTER submitted_at;
CREATE TABLE IF NOT EXISTS file_references (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    file_id BIGINT UNSIGNED NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id BIGINT UNSIGNED NOT NULL,
    purpose VARCHAR(50) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_file_reference (file_id, resource_type, resource_id, purpose),
    KEY idx_file_reference_resource (resource_type, resource_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
