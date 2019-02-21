CREATE TABLE IF NOT EXISTS hashes(
	post_id INT NOT NULL REFERENCES posts(id),
	sha256 CHAR(64),
	md5 CHAR(32)
);

CREATE TABLE IF NOT EXISTS post_description(
	post_id INT NOT NULL REFERENCES posts(id),
	itteration INT NOT NULL,
	text TEXT NOT NULL,
	PRIMARY KEY(post_id, itteration)
);

CREATE TABLE IF NOT EXISTS user_pools(
	id SERIAL PRIMARY KEY,
	user_id INT NOT NULL REFERENCES users(id),
	title VARCHAR(128) NOT NULL,
	description TEXT
);

CREATE TABLE IF NOT EXISTS pool_mappings(
	pool_id INT NOT NULL REFERENCES user_pools(id),
	post_id INT NOT NULL REFERENCES posts(id),
	position INT NOT NULL DEFAULT 0,
	PRIMARY KEY(pool_id, post_id)
);

CREATE TABLE IF NOT EXISTS post_info(
	post_id INT NOT NULL REFERENCES posts(id),
	width INT,
	height INT
);



ALTER TABLE posts ADD COLUMN IF NOT EXISTS file_size BIGINT NOT NULL DEFAULT 0;
