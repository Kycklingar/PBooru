CREATE TABLE search_count_cache(
	id SERIAL PRIMARY KEY,
	str TEXT NOT NULL UNIQUE,
	count INT NOT NULL
);

CREATE TABLE search_count_cache_tag_mapping(
	cache_id INT NOT NULL REFERENCES search_count_cache(id) ON DELETE CASCADE ON UPDATE CASCADE,
	tag_id INT NOT NULL REFERENCES tags(id)
);
