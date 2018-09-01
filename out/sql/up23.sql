CREATE TABLE IF NOT EXISTS post_comments(
    id INTEGER PRIMARY KEY,
    post_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY(post_id) REFERENCES posts(id),
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE INDEX pc_post_id ON post_comments(post_id);
CREATE INDEX pc_user_id ON post_comments(user_id);