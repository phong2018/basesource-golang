-- +goose Up
CREATE TABLE outbox_events (
    id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_id       VARCHAR(36)  NOT NULL UNIQUE,
    event_type     VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64)  NOT NULL,
    aggregate_id   VARCHAR(64)  NOT NULL,
    payload        JSON         NOT NULL,
    created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
