SELECT count(t1.post_id)
FROM post_tag_mappings t1 
JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 4
AND t1.tag_id = 2
AND t1.post_id NOT IN(SELECT post_id FROM post_tag_mappings WHERE tag_id = 209)
AND (
    SELECT deleted 
    FROM posts 
    WHERE id = t1.post_id
    ) = 0;

EXPLAIN
SELECT id FROM posts WHERE id IN(SELECT t1.post_id
FROM post_tag_mappings t1 
JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 4
AND t1.post_id NOT IN(SELECT post_id FROM post_tag_mappings WHERE tag_id = 209)
AND t1.tag_id = 2
AND (
    SELECT deleted
    FROM posts 
    WHERE id = t1.post_id
    ) = 0
    LIMIT 10
);

SELECT id FROM posts WHERE id IN(
    SELECT t1.post_id
    FROM post_tag_mappings t1 
    JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 4
    AND t1.post_id NOT IN(
        SELECT post_id
        FROM post_tag_mappings
        WHERE tag_id = 209
        )
    AND t1.tag_id = 2
    AND (
        SELECT deleted
        FROM posts 
        WHERE id = t1.post_id
    ) = 0
    ORDER BY t1.post_id DESC
    LIMIT 10
);


SELECT id, (SELECT nspace FROM namespaces WHERE id = t.namespace_id), tag FROM tags t WHERE tag = 'krystal';