#posts_1
#users_1

ALTER TABLE posts ADD COLUMN users INTEGER NOT NULL REFERENCES users(id);
