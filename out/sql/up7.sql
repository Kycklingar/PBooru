CREATE TABLE reports(
	id SERIAL PRIMARY KEY,
	post_id INT NOT NULL REFERENCES posts(id),
	reporter INT REFERENCES users(id),
	reason INT,
	description TEXT
);

