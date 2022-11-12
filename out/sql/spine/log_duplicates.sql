#spine

CREATE TABLE log_duplicates (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	id SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE log_duplicate_posts (
	id INTEGER NOT NULL REFERENCES log_duplicates(id),
	dup_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	PRIMARY KEY(id, dup_id)
);
