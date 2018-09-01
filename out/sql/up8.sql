CREATE TABLE users(
    id INTEGER PRIMARY KEY,
    username VARCHAR(64) UNIQUE NOT NULL,
    passwordhash CHAR(60) NOT NULL,
    salt CHAR(128) NOT NULL,
    datejoined TEXT NOT NULL,
    adminflag INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE sessions(
    user_id INTEGER NOT NULL,
    sesskey CHAR(64) NOT NULL,
    expire TEXT NOT NULL
);