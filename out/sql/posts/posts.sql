#mimes
#users
#namespaces

CREATE TABLE IF NOT EXISTS posts(
	id SERIAL PRIMARY KEY,
	multihash CHAR(59) UNIQUE NOT NULL,
	mime_id INT NOT NULL REFERENCES mime_type(id),
	removed BOOL NOT NULL DEFAULT FALSE,
	deleted BOOL NOT NULL DEFAULT FALSE,
	uploader INTEGER NOT NULL REFERENCES users(id),
	file_size BIGINT NOT NULL DEFAULT 0,
	score INTEGER NOT NULL DEFAULT 0,
	alt_group INT REFERENCES posts(id),
	creation_date DATE DEFAULT NULL,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	description TEXT NOT NULL DEFAULT ''
);

CREATE INDEX ON posts(removed);
CREATE INDEX ON posts(alt_group);

CREATE TABLE IF NOT EXISTS post_info(
	post_id INT NOT NULL REFERENCES posts(id),
	width INT,
	height INT
);

CREATE TABLE IF NOT EXISTS post_comments(
	id SERIAL PRIMARY KEY,
	post_id INT NOT NULL REFERENCES posts(id),
	user_id INT NOT NULL REFERENCES users(id),
	text TEXT NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS hashes(
	post_id INT NOT NULL REFERENCES posts(id),
	sha256 CHAR(64),
	md5 CHAR(32)
);

-- FILENAMES, SOURCES, VERSION, ETC
CREATE TABLE IF NOT EXISTS post_metadata (
	id BIGSERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	namespace_id INTEGER NOT NULL REFERENCES namespaces(id),
	metadata TEXT NOT NULL,
	UNIQUE(post_id, namespace_id, metadata)
);

CREATE TABLE post_creation_dates (
	id SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	created DATE NOT NULL,
	UNIQUE(post_id, created)
);

