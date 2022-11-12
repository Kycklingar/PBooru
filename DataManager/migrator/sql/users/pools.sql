#users
#posts

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

