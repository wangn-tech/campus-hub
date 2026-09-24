CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    nickname VARCHAR(50) NOT NULL,
    avatar_url VARCHAR(500) NOT NULL DEFAULT '',
    introduction VARCHAR(500) NOT NULL DEFAULT '',
    gender TINYINT UNSIGNED NOT NULL DEFAULT 0,
    birthday DATE NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    last_login_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    KEY idx_users_status_deleted (status, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS roles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(50) NOT NULL,
    description VARCHAR(200) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_roles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    role_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_user_role (user_id, role_id),
    KEY idx_user_roles_role (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_credit_profiles (
    user_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    score INT NOT NULL DEFAULT 100,
    level TINYINT NOT NULL DEFAULT 4,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_credit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    user_id BIGINT UNSIGNED NOT NULL,
    change_type TINYINT NOT NULL,
    source_type VARCHAR(50) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    before_score INT NOT NULL,
    after_score INT NOT NULL,
    delta INT NOT NULL,
    reason VARCHAR(255) NOT NULL DEFAULT '',
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_credit_source (user_id, source_type, source_id),
    KEY idx_credit_user_created (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS categories (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    name VARCHAR(50) NOT NULL UNIQUE,
    icon VARCHAR(100) NOT NULL DEFAULT '',
    sort INT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 1,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS tags (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    name VARCHAR(50) NOT NULL UNIQUE,
    slug VARCHAR(80) NOT NULL UNIQUE,
    color VARCHAR(20) NOT NULL DEFAULT '',
    icon VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(200) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 1,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS tag_scopes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tag_id BIGINT UNSIGNED NOT NULL,
    scope VARCHAR(20) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    usage_count INT UNSIGNED NOT NULL DEFAULT 0,
    view_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_tag_scope (tag_id, scope),
    KEY idx_scope_status_usage (scope, status, usage_count)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_interest_relations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    tag_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_user_tag (user_id, tag_id),
    KEY idx_interest_tag (tag_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS activities (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    title VARCHAR(100) NOT NULL,
    cover_file_id BIGINT UNSIGNED NULL,
    cover_url VARCHAR(500) NOT NULL DEFAULT '',
    description MEDIUMTEXT NULL,
    category_id BIGINT UNSIGNED NOT NULL,
    organizer_id BIGINT UNSIGNED NOT NULL,
    organizer_name VARCHAR(50) NOT NULL DEFAULT '',
    organizer_avatar VARCHAR(500) NOT NULL DEFAULT '',
    contact_phone VARCHAR(20) NOT NULL DEFAULT '',
    register_start_at DATETIME(3) NOT NULL,
    register_end_at DATETIME(3) NOT NULL,
    activity_start_at DATETIME(3) NOT NULL,
    activity_end_at DATETIME(3) NOT NULL,
    location VARCHAR(200) NOT NULL DEFAULT '',
    address_detail VARCHAR(500) NOT NULL DEFAULT '',
    longitude DECIMAL(10,7) NULL,
    latitude DECIMAL(10,7) NULL,
    max_participants INT UNSIGNED NOT NULL DEFAULT 0,
    approved_participant_count INT UNSIGNED NOT NULL DEFAULT 0,
    pending_participant_count INT UNSIGNED NOT NULL DEFAULT 0,
    require_approval TINYINT(1) NOT NULL DEFAULT 0,
    require_student_verify TINYINT(1) NOT NULL DEFAULT 0,
    min_credit_score INT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 0,
    reject_reason VARCHAR(500) NOT NULL DEFAULT '',
    view_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
    version INT UNSIGNED NOT NULL DEFAULT 0,
    published_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    KEY idx_activity_category_status (category_id, status, activity_start_at),
    KEY idx_activity_status_time (status, activity_start_at, activity_end_at),
    KEY idx_activity_organizer_status (organizer_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS activity_tag_relations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    activity_id BIGINT UNSIGNED NOT NULL,
    tag_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_activity_tag (activity_id, tag_id),
    KEY idx_activity_tag_tag (tag_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS activity_registrations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    activity_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    status TINYINT NOT NULL DEFAULT 0,
    active_flag TINYINT NULL,
    expires_at DATETIME(3) NULL,
    reviewed_by BIGINT UNSIGNED NULL,
    decided_at DATETIME(3) NULL,
    cancel_time DATETIME(3) NULL,
    reject_reason VARCHAR(255) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    UNIQUE KEY uk_active_registration (activity_id, user_id, active_flag),
    KEY idx_registration_user_status (user_id, status, created_at),
    KEY idx_registration_activity_status (activity_id, status),
    KEY idx_registration_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS tickets (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    code VARCHAR(32) NOT NULL UNIQUE,
    registration_id BIGINT UNSIGNED NOT NULL UNIQUE,
    activity_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    status TINYINT NOT NULL DEFAULT 0,
    valid_start_at DATETIME(3) NULL,
    valid_end_at DATETIME(3) NULL,
    issued_at DATETIME(3) NOT NULL,
    used_at DATETIME(3) NULL,
    voided_at DATETIME(3) NULL,
    version INT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    KEY idx_ticket_user_status (user_id, status),
    KEY idx_ticket_activity_status (activity_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS check_ins (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    ticket_id BIGINT UNSIGNED NOT NULL UNIQUE,
    registration_id BIGINT UNSIGNED NOT NULL,
    activity_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    operator_id BIGINT UNSIGNED NOT NULL,
    client_request_id VARCHAR(64) NOT NULL UNIQUE,
    longitude DECIMAL(10,7) NULL,
    latitude DECIMAL(10,7) NULL,
    checked_in_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_checkin_activity_time (activity_id, checked_in_at),
    KEY idx_checkin_user_time (user_id, checked_in_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    user_id BIGINT UNSIGNED NOT NULL,
    type VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    data JSON NULL,
    is_read TINYINT(1) NOT NULL DEFAULT 0,
    read_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_notification_user_created (user_id, created_at),
    KEY idx_notification_user_read (user_id, is_read)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    event_id CHAR(36) NOT NULL UNIQUE,
    event_type VARCHAR(100) NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    aggregate_version INT UNSIGNED NOT NULL DEFAULT 1,
    partition_key VARCHAR(100) NOT NULL DEFAULT '',
    payload JSON NOT NULL,
    headers JSON NULL,
    status TINYINT NOT NULL DEFAULT 0,
    retry_count INT NOT NULL DEFAULT 0,
    next_retry_at DATETIME(3) NULL,
    sent_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    KEY idx_outbox_status_retry (status, next_retry_at),
    KEY idx_outbox_aggregate (aggregate_type, aggregate_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS idempotency_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    request_id VARCHAR(64) NOT NULL UNIQUE,
    user_id BIGINT UNSIGNED NOT NULL,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    response_status INT NOT NULL DEFAULT 0,
    response_body JSON NULL,
    expires_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    KEY idx_idempotency_user_expires (user_id, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO roles (code, name, description, created_at, updated_at)
VALUES ('user', '普通用户', '普通用户角色', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
       ('admin', '管理员', '平台管理员角色', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE name = VALUES(name), updated_at = VALUES(updated_at);
