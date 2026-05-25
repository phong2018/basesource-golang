-- +goose Up
CREATE TABLE outbox_deliveries (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    outbox_event_id BIGINT UNSIGNED NOT NULL,
    destination     VARCHAR(32)  NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    attempt_count   INT UNSIGNED NOT NULL DEFAULT 0,
    last_error      TEXT,
    published_at    DATETIME(3),
    created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    CONSTRAINT fk_od_event FOREIGN KEY (outbox_event_id) REFERENCES outbox_events(id),
    INDEX idx_od_dest_status (destination, status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS outbox_deliveries;
