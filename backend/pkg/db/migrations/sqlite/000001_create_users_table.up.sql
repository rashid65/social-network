-- users table
CREATE TABLE users (
    id              TEXT      PRIMARY KEY,
    email           TEXT      NOT NULL UNIQUE,
    password_hash   TEXT      NOT NULL,
    first_name      TEXT      NOT NULL,
    last_name       TEXT      NOT NULL,
    date_of_birth   TEXT      NULL,            -- stored as ISO‑8601 string
    nickname        TEXT      NULL UNIQUE,
    about_me        TEXT      NULL,
    avatar_path     TEXT      NULL,
    is_public       INTEGER   NOT NULL DEFAULT 1,  -- 1 = public, 0 = private
    created_at      TEXT      NOT NULL DEFAULT CURRENT_TIMESTAMP
);