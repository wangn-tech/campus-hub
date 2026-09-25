CREATE TABLE IF NOT EXISTS activity_status_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid CHAR(36) NOT NULL UNIQUE,
    activity_id BIGINT UNSIGNED NOT NULL,
    from_status TINYINT NOT NULL,
    to_status TINYINT NOT NULL,
    operator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    operator_type TINYINT NOT NULL DEFAULT 1,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL,
    KEY idx_activity_created (activity_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO categories (uuid, name, icon, sort, status, created_at, updated_at) VALUES
    ('aaaaaaaa-0000-4000-8000-000000000001', '学术讲座', 'lecture', 1, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('aaaaaaaa-0000-4000-8000-000000000002', '文体活动', 'sports', 2, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('aaaaaaaa-0000-4000-8000-000000000003', '志愿服务', 'volunteer', 3, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('aaaaaaaa-0000-4000-8000-000000000004', '社团招新', 'club', 4, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('aaaaaaaa-0000-4000-8000-000000000005', '竞赛比赛', 'competition', 5, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('aaaaaaaa-0000-4000-8000-000000000006', '其他', 'other', 99, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE icon = VALUES(icon), sort = VALUES(sort), status = VALUES(status), updated_at = VALUES(updated_at);

INSERT INTO tags (uuid, name, slug, color, icon, description, status, created_at, updated_at) VALUES
    ('bbbbbbbb-0000-4000-8000-000000000001', '讲座', 'lecture', '#4A90D9', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000002', '运动', 'sports', '#52C41A', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000003', '音乐', 'music', '#EB2F96', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000004', '摄影', 'photography', '#722ED1', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000005', '编程', 'coding', '#13C2C2', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000006', '公益', 'charity', '#FA8C16', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000007', '户外', 'outdoor', '#A0D911', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000008', '文艺', 'arts', '#F5222D', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-000000000009', '求职', 'career', '#2F54EB', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-00000000000a', '社交', 'social', '#FAAD14', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-00000000000b', '竞赛', 'competition', '#D4380D', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)),
    ('bbbbbbbb-0000-4000-8000-00000000000c', '阅读', 'reading', '#08979C', '', '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE color = VALUES(color), icon = VALUES(icon), status = VALUES(status), updated_at = VALUES(updated_at);

INSERT INTO tag_scopes (tag_id, scope, status, usage_count, view_count, created_at, updated_at)
SELECT id, 'activity', 1, 0, 0, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)
FROM tags
WHERE uuid IN (
    'bbbbbbbb-0000-4000-8000-000000000001', 'bbbbbbbb-0000-4000-8000-000000000002',
    'bbbbbbbb-0000-4000-8000-000000000003', 'bbbbbbbb-0000-4000-8000-000000000004',
    'bbbbbbbb-0000-4000-8000-000000000005', 'bbbbbbbb-0000-4000-8000-000000000006',
    'bbbbbbbb-0000-4000-8000-000000000007', 'bbbbbbbb-0000-4000-8000-000000000008',
    'bbbbbbbb-0000-4000-8000-000000000009', 'bbbbbbbb-0000-4000-8000-00000000000a',
    'bbbbbbbb-0000-4000-8000-00000000000b', 'bbbbbbbb-0000-4000-8000-00000000000c'
)
ON DUPLICATE KEY UPDATE status = 1, updated_at = VALUES(updated_at);

INSERT INTO tag_scopes (tag_id, scope, status, usage_count, view_count, created_at, updated_at)
SELECT id, 'interest', 1, 0, 0, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)
FROM tags
WHERE uuid IN (
    'bbbbbbbb-0000-4000-8000-000000000001', 'bbbbbbbb-0000-4000-8000-000000000002',
    'bbbbbbbb-0000-4000-8000-000000000003', 'bbbbbbbb-0000-4000-8000-000000000004',
    'bbbbbbbb-0000-4000-8000-000000000005', 'bbbbbbbb-0000-4000-8000-000000000006',
    'bbbbbbbb-0000-4000-8000-000000000007', 'bbbbbbbb-0000-4000-8000-00000000000c'
)
ON DUPLICATE KEY UPDATE status = 1, updated_at = VALUES(updated_at);
