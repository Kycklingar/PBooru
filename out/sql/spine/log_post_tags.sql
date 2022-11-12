#spine
#tags

CREATE TABLE log_post_tags (
	id BIGSERIAL PRIMARY KEY,
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE log_post_tags_map (
	ptid BIGINT REFERENCES log_post_tags(id) ON DELETE CASCADE,
	tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE log_multi_post_tags (
	id SERIAL PRIMARY KEY,
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
	UNIQUE(log_id, action, tag_id)
);

CREATE TABLE log_multi_posts_affected (
	id INTEGER NOT NULL REFERENCES log_multi_post_tags(id) ON DELETE CASCADE,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	UNIQUE(id, post_id)
);

CREATE INDEX ON log_post_tags (post_id);
CREATE INDEX ON log_post_tags (log_id);
CREATE INDEX ON log_post_tags_map (ptid);
