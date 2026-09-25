CREATE TABLE IF NOT EXISTS student_verifications (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    user_id BIGINT UNSIGNED NOT NULL UNIQUE,
    status TINYINT NOT NULL DEFAULT 0,
    real_name_encrypted VARCHAR(255) NULL,
    school_name VARCHAR(100) NULL,
    student_id_encrypted VARCHAR(255) NULL,
    student_id_hash VARCHAR(64) NOT NULL DEFAULT '',
    department VARCHAR(100) NULL,
    admission_year VARCHAR(10) NULL,
    front_file_id BIGINT UNSIGNED NULL,
    back_file_id BIGINT UNSIGNED NULL,
    reject_reason VARCHAR(255) NOT NULL DEFAULT '',
    cancel_reason VARCHAR(255) NOT NULL DEFAULT '',
    reviewer_id BIGINT UNSIGNED NULL,
    submitted_at DATETIME(3) NULL,
    reviewed_at DATETIME(3) NULL,
    verified_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    KEY idx_verification_status_updated (status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS student_verification_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    verification_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    from_status TINYINT NOT NULL,
    to_status TINYINT NOT NULL,
    event_type VARCHAR(80) NOT NULL,
    operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    operator_type TINYINT NOT NULL DEFAULT 1,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    metadata JSON NULL,
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    KEY idx_verification_event_created (verification_id, created_at),
    KEY idx_verification_user_created (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS chat_groups (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    activity_id BIGINT UNSIGNED NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    owner_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS chat_group_members (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    group_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    role TINYINT NOT NULL DEFAULT 1,
    status TINYINT NOT NULL DEFAULT 1,
    joined_at DATETIME(3) NOT NULL,
    left_at DATETIME(3) NULL,
    last_read_message_id BIGINT UNSIGNED NULL,
    muted TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_group_member (group_id, user_id),
    KEY idx_member_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS chat_messages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    group_id BIGINT UNSIGNED NOT NULL,
    sender_id BIGINT UNSIGNED NOT NULL,
    client_message_id VARCHAR(64) NOT NULL,
    msg_type TINYINT NOT NULL,
    content TEXT NULL,
    image_file_id BIGINT UNSIGNED NULL,
    status TINYINT NOT NULL DEFAULT 1,
    recalled_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_message_sender_client (sender_id, client_message_id),
    KEY idx_message_group_created (group_id, created_at),
    KEY idx_message_sender_created (sender_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS files (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    storage_driver VARCHAR(20) NOT NULL,
    bucket VARCHAR(100) NOT NULL,
    object_key VARCHAR(500) NOT NULL,
    url VARCHAR(500) NOT NULL,
    origin_name VARCHAR(255) NOT NULL DEFAULT '',
    biz_type VARCHAR(32) NOT NULL,
    file_size BIGINT UNSIGNED NOT NULL DEFAULT 0,
    mime_type VARCHAR(64) NOT NULL DEFAULT '',
    extension VARCHAR(10) NOT NULL DEFAULT '',
    sha256 VARCHAR(64) NOT NULL DEFAULT '',
    uploader_id BIGINT UNSIGNED NOT NULL,
    status TINYINT NOT NULL DEFAULT 0,
    metadata JSON NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    KEY idx_file_uploader (uploader_id),
    KEY idx_file_biz_status (biz_type, status),
    KEY idx_file_sha256 (sha256)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    actor_type TINYINT NOT NULL DEFAULT 1,
    action VARCHAR(80) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    result TINYINT NOT NULL DEFAULT 1,
    error_code VARCHAR(50) NOT NULL DEFAULT '',
    ip VARCHAR(45) NOT NULL DEFAULT '',
    user_agent VARCHAR(255) NOT NULL DEFAULT '',
    metadata JSON NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_audit_actor_created (actor_id, created_at),
    KEY idx_audit_resource_created (resource_type, resource_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
