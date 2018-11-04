CREATE TABLE IF NOT EXISTS dbinfo(
    ver INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS mime_type(
    id SERIAL PRIMARY KEY,
    mime VARCHAR(64),
    type VARCHAR(64)
);

CREATE TABLE IF NOT EXISTS users(
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    passwordhash CHAR(60) NOT NULL,
    salt CHAR(128) NOT NULL,
    datejoined TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    adminflag INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS posts(
    id SERIAL PRIMARY KEY,
    multihash CHAR(49) UNIQUE NOT NULL,
    thumbhash CHAR(49) NOT NULL,
    mime_id INT NOT NULL REFERENCES mime_type(id),
    deleted BOOL NOT NULL DEFAULT FALSE,
    uploader INTEGER NOT NULL REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS namespaces(
    id SERIAL PRIMARY KEY,
    nspace VARCHAR(128) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS tags(
    id SERIAL PRIMARY KEY,
    tag VARCHAR(256) NOT NULL,
    namespace_id INTEGER NOT NULL REFERENCES namespaces(id),
    count INT NOT NULL DEFAULT 0,
    CONSTRAINT tag_namespace UNIQUE (tag, namespace_id)
);

CREATE TABLE IF NOT EXISTS alias(
    alias_from INT NOT NULL REFERENCES tags(id),
    alias_to INT NOT NULL REFERENCES tags(id),
    CONSTRAINT alias_to_from UNIQUE (alias_from, alias_to)
);

CREATE TABLE IF NOT EXISTS comics(
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comic_chapter(
    id SERIAL PRIMARY KEY,
    comic_id INT NOT NULL REFERENCES comics(id),
    c_order INT NOT NULL,
    title TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comic_mappings(
    id SERIAL PRIMARY KEY,
    comic_id INT NOT NULL REFERENCES comics(id),
    post_id INT NOT NULL REFERENCES posts(id),
    post_order INT NOT NULL,
    chapter_id INT NOT NULL REFERENCES comic_chapter(id)
);

CREATE TABLE IF NOT EXISTS comment_wall(
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    text TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS counter(
    count INT NOT NULL
);

CREATE TABLE IF NOT EXISTS duplicate_posts(
    id SERIAL PRIMARY KEY,
    dup_id INT NOT NULL REFERENCES posts(id),
    post_id INT NOT NULL REFERENCES posts(id),
    level INT NOT NULL
);

CREATE TABLE IF NOT EXISTS tag_history(
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    post_id INT NOT NULL REFERENCES posts(id),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS edited_tags(
    id SERIAL PRIMARY KEY,
    history_id INT NOT NULL REFERENCES tag_history(id),
    tag_id INT NOT NULL REFERENCES tags(id),
    direction INT NOT NULL
);

CREATE TABLE IF NOT EXISTS parent_tags(
    parent_id INT NOT NULL REFERENCES tags(id),
    child_id INT NOT NULL REFERENCES tags(id),
    CONSTRAINT parent_child_unique UNIQUE (child_id, parent_id)
);

CREATE TABLE IF NOT EXISTS phash(
    post_id INT PRIMARY KEY REFERENCES posts(id),
    h1 INT NOT NULL,
    h2 INT NOT NULL,
    h3 INT NOT NULL,
    h4 INT NOT NULL
);

CREATE TABLE IF NOT EXISTS post_comments(
    id SERIAL PRIMARY KEY,
    post_id INT NOT NULL REFERENCES posts(id),
    user_id INT NOT NULL REFERENCES users(id),
    text TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS post_tag_mappings(
    tag_id INT NOT NULL REFERENCES tags(id),
    post_id INT NOT NULL REFERENCES posts(id),
    CONSTRAINT ptm_unique UNIQUE (tag_id, post_id)
);

CREATE TABLE IF NOT EXISTS sessions(
    user_id INT NOT NULL REFERENCES users(id),
    sesskey CHAR(64) NOT NULL UNIQUE,
    expire TIMESTAMP NOT NULL
);

INSERT INTO dbinfo VALUES(0);