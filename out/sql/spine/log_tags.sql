#spine
#tags

CREATE TABLE log_tag_alias (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	alias_from INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
	alias_to INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE log_tag_parent (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	action log_action NOT NULL,
	parent INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
	child INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE
);
