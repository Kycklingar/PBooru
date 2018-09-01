DROP TABLE sessions;

CREATE TABLE sessions(
    user_id INTEGER NOT NULL,
    sesskey CHAR(64) UNIQUE NOT NULL,
    expire TEXT NOT NULL
);