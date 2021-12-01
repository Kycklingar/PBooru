CREATE TABLE IF NOT EXISTS dns_creator (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS dns_domain (
	id SERIAL PRIMARY KEY,
	domain TEXT NOT NULL,
	icon TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS dns_creator_urls (
	id INTEGER NOT NULL REFERENCES dns_creator(id) ON DELETE CASCADE,
	domain INTEGER NOT NULL REFERENCES dns_domain(id) ON DELETE CASCADE,
	url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS dns_tag (
	id VARCHAR(10) PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	score INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS dns_tags (
	creator_id INTEGER REFERENCES dns_creator(id) ON DELETE CASCADE,
	tag_id VARCHAR(10) REFERENCES dns_tag(id) ON DELETE CASCADE ON UPDATE CASCADE 
);

CREATE TABLE IF NOT EXISTS dns_tag_mapping (
	tag_id INTEGER NOT NULL REFERENCES tags(id),
	creator_id INTEGER NOT NULL REFERENCES dns_creator(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS dns_banners (
	creator_id INTEGER NOT NULL REFERENCES dns_creator(id),
	cid TEXT NOT NULL,
	banner_type TEXT NOT NULL
);

CREATE OR REPLACE VIEW dns_creator_scores AS
	SELECT c.id, c.name, COALESCE(SUM(dt.score), 0) AS score
	FROM dns_creator c
	LEFT JOIN dns_tags dts ON c.id = dts.creator_id
	LEFT JOIN dns_tag dt ON dts.tag_id = dt.id
	GROUP BY c.id;

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
	post_id INTEGER NOT NULL REFERENCES posts(id),
	created DATE NOT NULL
);

CREATE TYPE log_action AS ENUM ('create', 'modify', 'delete');

CREATE TABLE logs (
	log_id BIGSERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id),
	--tables_affected TEXT NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE logs_affected (
	log_id BIGSERIAL REFERENCES logs(logs_id),
	log_table TEXT NOT NULL,

	UNIQUE(log_id, log_table)
);

CREATE TABLE log_post_description (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	post_id INTEGER NOT NULL,
	description TEXT NOT NULL
);

CREATE TABLE log_post_metadata (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,

	post_id INTEGER NOT NULL,
	namespace TEXT NOT NULL,
	metadata TEXT NOT NULL
);

CREATE TABLE log_post_creation_dates (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,

	post_id INTEGER NOT NULL,
	created DATE NOT NULL
);

CREATE TABLE log_post_tags (
	id BIGSERIAL PRIMARY KEY,
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	post_id INTEGER NOT NULL,
);

CREATE TABLE log_post_tags_map (
	ptid BIGINT REFERENCES log_post_tags(id) ON DELETE CASCADE,
	tag_id INTEGER NOT NULL
);
