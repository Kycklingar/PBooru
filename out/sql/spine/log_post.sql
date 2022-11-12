#spine
#posts
#namespaces

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
	namespace_id INTEGER NOT NULL REFERENCES namespaces(id),
	metadata TEXT NOT NULL
);

CREATE TABLE log_post_creation_dates (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,

	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	created DATE NOT NULL
);

CREATE TABLE log_post_alts (
	al_id SERIAL PRIMARY KEY,
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE
);

CREATE TABLE log_post_alt_posts(
	al_id INTEGER NOT NULL REFERENCES log_post_alts(al_id) ON DELETE CASCADE,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

