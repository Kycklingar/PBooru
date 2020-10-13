--CREATE TABLE IF NOT EXISTS forum_category {
--	id SERIAL PRIMARY KEY,
--	name TEXT
--};

CREATE TABLE IF NOT EXISTS forum_board (
	id SERIAL PRIMARY KEY,
	uri TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL
	--category INT REFERENCES forum_category (id)
);

CREATE TABLE IF NOT EXISTS forum_post (
	id SERIAL PRIMARY KEY,
	board_id INT REFERENCES forum_board(id) NOT NULL,
	rid INT NOT NULL,
	poster INT REFERENCES users(id),
	title TEXT,
	body TEXT,
	created TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
	reply_to INT REFERENCES forum_post(id) ON DELETE CASCADE,

	CONSTRAINT board_rid_unique UNIQUE (board_id, rid)
);

--CREATE TABLE IF NOT EXISTS forum_file (
--	post_id INT REFERENCES forum_post (id),
--	cid TEXT NOT NULL
--);

