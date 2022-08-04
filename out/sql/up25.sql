-- FILENAMES, SOURCES, VERSION, ETC
CREATE TABLE IF NOT EXISTS post_metadata (
	id BIGSERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	namespace_id INTEGER NOT NULL REFERENCES namespaces(id),
	metadata TEXT NOT NULL,
	UNIQUE(post_id, namespace_id, metadata)
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

CREATE TABLE logs_tables (
	id SERIAL PRIMARY KEY,
	table_name TEXT NOT NULL UNIQUE
);

CREATE TABLE logs_tables_altered (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	table_id INTEGER NOT NULL REFERENCES logs_tables(id) ON DELETE CASCADE,

	UNIQUE(log_id, table_id)
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
	namespace_id INTEGER NOT NULL REFERENCES namespaces(id),
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
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE
	--new_alt INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE log_post_alt_posts(
	al_id INTEGER NOT NULL REFERENCES log_post_alts(al_id) ON DELETE CASCADE,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE log_tag_alias (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	alias_from INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
	alias_to INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE log_tag_parent (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	parent INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
	child INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE
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

CREATE TABLE log_comics (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	id INTEGER NOT NULL,
	action log_action NOT NULL,
	title TEXT NOT NULL
);

CREATE TABLE log_chapters (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	comic_id INTEGER NOT NULL,
	chapter_id INTEGER NOT NULL,
	c_order INTEGER NOT NULL,
	title TEXT NOT NULL
);

ALTER TABLE comic_mappings RENAME TO comic_page;
ALTER TABLE comic_page RENAME COLUMN post_order TO page;
ALTER TABLE log_comic_page RENAME TO log_comic_page_old;

CREATE TABLE log_comic_page (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	comic_page_id INTEGER NOT NULL,
	chapter_id INTEGER NOT NULL,
	post_id INTEGER NOT NULL,
	page INTEGER NOT NULL
);

CREATE VIEW view_log_comic_page_diff AS
SELECT
	new.comic_page_id, new.action, new.log_id,
	new.chapter_id, new.post_id, new.page,
	old.log_id AS old_log_id,
	old.chapter_id AS old_chapter_id,
	old.post_id AS old_post_id,
	old.page AS old_page
FROM log_comic_page new
LEFT JOIN log_comic_page old
ON new.comic_page_id = old.comic_page_id
AND old.log_id = (
	SELECT MAX(log_id)
	FROM log_comic_page
	WHERE log_id < new.log_id
	AND comic_page_id = new.comic_page_id
);

CREATE TABLE log_duplicates (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	id INTEGER SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE
);

CREATE TABLE log_duplicate_posts (
	id INTEGER NOT NULL REFERENCES log_duplicates(id),
	dup_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	PRIMARY KEY(id, dup_id)
);

