CREATE TABLE passwords (
	user_id INTEGER NOT NULL REFERENCES users(id),
	hash CHARACTER(60) NOT NULL,
	salt CHARACTER(128) NOT NULL
);

INSERT INTO passwords
	SELECT id, passwordhash, salt
	FROM users;


ALTER TABLE users
DROP COLUMN passwordhash,
DROP COLUMN salt;
