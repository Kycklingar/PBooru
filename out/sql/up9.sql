CREATE TABLE edited_tags(
    id INTEGER PRIMARY KEY,
    history_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    direction INTEGER NOT NULL,
    FOREIGN KEY(tag_id) REFERENCES tags(id),
    FOREIGN KEY(history_id) REFERENCES tag_history(id)
);

CREATE TABLE tag_history(
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    post_id INTEGER NOT NULL,
    timestamp TEXT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id),
    FOREIGN KEY(post_id) REFERENCES posts(id)
);