CREATE TABLE IF NOT EXISTS forum_category (
	id SERIAL PRIMARY KEY,
	name TEXT,
	CONSTRAINT category_name_unique UNIQUE(name)
);

CREATE TABLE IF NOT EXISTS forum_board (
	--id SERIAL PRIMARY KEY,
	uri TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	category INT REFERENCES forum_category (id),
	top INT NOT NULL DEFAULT 0,

	thread_limit INT NOT NULL DEFAULT 60,

	--Board characteristics
	--cycle BOOLEAN DEFAULT FALSE,

	CONSTRAINT board_uri_unique UNIQUE (uri)
);

CREATE TABLE IF NOT EXISTS forum_thread (
	id SERIAL PRIMARY KEY,
	board TEXT REFERENCES forum_board(uri) ON DELETE CASCADE NOT NULL,

	start_post INT REFERENCES forum_post(id) ON DELETE CASCADE,

	--locked BOOLEAN NOT NULL DEFAULT false,

	bump_limit INT NOT NULL,
	bump_count INT NOT NULL DEFAULT 0,
	bumped TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS forum_post (
	id SERIAL PRIMARY KEY,
	thread_id INT REFERENCES forum_thread(id) ON DELETE CASCADE NOT NULL,
	rid INT NOT NULL,
	poster INT REFERENCES users(id),
	title TEXT,
	body TEXT,
	created TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS forum_replies (
	post_id INT NOT NULL REFERENCES forum_post(id) ON DELETE CASCADE,
	ref INT NOT NULL REFERENCES forum_post(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS forum_file (
	post_id INT REFERENCES forum_post (id) ON DELETE CASCADE,
	cid TEXT NOT NULL,
	filename TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS forum_postref (
	forum_post_id INT NOT NULL REFERENCES forum_post(id) ON DELETE CASCADE,
	post_id INT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
);

