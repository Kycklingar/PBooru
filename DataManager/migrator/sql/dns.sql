#tags

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

