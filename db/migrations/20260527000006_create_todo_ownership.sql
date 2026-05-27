-- +goose Up
ALTER TABLE todos
    ADD COLUMN owner_id        BIGINT UNSIGNED NULL,
    ADD COLUMN deleted_at      DATETIME NULL,
    ADD COLUMN attachment_url  VARCHAR(500) NULL,
    ADD CONSTRAINT fk_todos_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL;

CREATE TABLE todo_shares (
    todo_id    INT UNSIGNED NOT NULL,
    user_id    BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (todo_id, user_id),
    FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE todo_comments (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    todo_id    INT UNSIGNED NOT NULL,
    user_id    BIGINT UNSIGNED NOT NULL,
    body       TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS todo_comments;
DROP TABLE IF EXISTS todo_shares;
ALTER TABLE todos
    DROP FOREIGN KEY fk_todos_owner,
    DROP COLUMN owner_id,
    DROP COLUMN deleted_at,
    DROP COLUMN attachment_url;
