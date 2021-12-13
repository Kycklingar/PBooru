-- FILENAMES, SOURCES, VERSION, ETC
CREATE TABLE IF NOT EXISTS post_metadata (
	id BIGSERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	namespace TEXT NOT NULL,
	metadata TEXT NOT NULL,
	UNIQUE(post_id, namespace, metadata)
);

DROP TABLE post_description;

ALTER TABLE posts ADD COLUMN creation_date DATE DEFAULT NULL;
ALTER TABLE posts ADD COLUMN description TEXT NOT NULL DEFAULT '';

CREATE TABLE post_creation_dates (
	id SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	created DATE NOT NULL,
	UNIQUE(post_id, created)
);

CREATE TYPE log_action AS ENUM ('create', 'modify', 'delete');

CREATE TABLE logs (
	log_id BIGSERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id),
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE logs_affected (
	log_id BIGSERIAL REFERENCES logs(log_id),
	log_table TEXT NOT NULL,

	UNIQUE(log_id, log_table)
);

CREATE TABLE log_post(
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	UNIQUE(log_id, post_id)
);

CREATE TABLE log_post_description (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	description TEXT NOT NULL
);

CREATE TABLE log_post_metadata (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,

	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	namespace TEXT NOT NULL,
	metadata TEXT NOT NULL
);

CREATE TABLE log_post_creation_dates (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,

	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	created DATE NOT NULL
);

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

CREATE TABLE log_post_alts (
	al_id SERIAL PRIMARY KEY,
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	new_alt INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE log_post_alt_posts(
	al_id INTEGER NOT NULL REFERENCES log_post_alts(al_id) ON DELETE CASCADE,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

