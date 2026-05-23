-- +goose Up
CREATE TABLE audit_logs (
    id         INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    entity     VARCHAR(50)  NOT NULL,
    entity_id  INT UNSIGNED NOT NULL,
    action     VARCHAR(20)  NOT NULL,
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_audit_logs_entity_id (entity, entity_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE audit_logs;
